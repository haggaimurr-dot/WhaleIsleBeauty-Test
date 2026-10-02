package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"log"
	"net/http"
	"time"
	"unicode/utf8"
)

// 订阅消息。客人在小程序里同意一次，后端才能发一条（一次性订阅），模板和字段对应见 src/utils/subscribe.ts。
//
//   - 预约确认：店主点“确认”后马上发
//   - 到店提醒：已确认的预约，开始前一天 reminderHour 点以后由定时任务发
//
// 发送结果记在 notices 表里，同一条预约的同一个时段每种消息只发一次；改期后时段变了会重新发。
// 发不出去（客人没订阅、拒绝了）不影响预约本身。

const (
	tmplConfirmed = "R6MUu_p5k6R-LPbVgm60btOsqWDVdb3Rbpykw8hH08Q" // 预约成功通知
	tmplReminder  = "TpVpZ-EaOftX3_nW3RFYtx1BiBwpZbJRtJEfifCeM2Y" // 日程安排提醒

	noticeConfirmed = "confirmed"
	noticeReminder  = "reminder"

	reminderHour = 18 // 前一天 18:00 以后发到店提醒
	msgPage      = "pages/me/index"
)

// SubscribeMsg 对应微信 subscribeMessage.send 的请求体
type SubscribeMsg struct {
	ToUser     string                       `json:"touser"`
	TemplateID string                       `json:"template_id"`
	Page       string                       `json:"page,omitempty"`
	State      string                       `json:"miniprogram_state,omitempty"`
	Data       map[string]map[string]string `json:"data"`
}

type Notifier interface {
	Send(ctx context.Context, m SubscribeMsg) error
}

// WxError 微信接口返回的错误码。43101 是客人没订阅或者拒绝了
type WxError struct {
	Code int    `json:"errcode"`
	Msg  string `json:"errmsg"`
}

func (e *WxError) Error() string { return fmt.Sprintf("wx errcode=%d %s", e.Code, e.Msg) }

// retryable 网络错误和微信“系统繁忙”下次再试；其他错误（没订阅、参数不对）重试也没用
func retryable(err error) bool {
	var we *WxError
	return !errors.As(err, &we) || we.Code == -1
}

// wxNotifier 走云托管的开放接口服务：容器内用 http 调 api.weixin.qq.com 不需要 access_token。
// 要在云托管控制台“云调用 → 微信令牌权限配置”里加上 /cgi-bin/message/subscribe/send
type wxNotifier struct {
	base   string // 默认 http://api.weixin.qq.com，测试时换成本地服务
	state  string // 点消息打开哪个版本：formal / trial / developer
	client *http.Client
}

func (n *wxNotifier) Send(ctx context.Context, m SubscribeMsg) error {
	m.State = n.state
	body, _ := json.Marshal(m)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.base+"/cgi-bin/message/subscribe/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var we WxError
	if err := json.NewDecoder(resp.Body).Decode(&we); err != nil {
		return fmt.Errorf("wx status=%d: %w", resp.StatusCode, err)
	}
	if we.Code != 0 {
		return &we
	}
	return nil
}

// logNotifier 本地开发用：只打日志，不发
type logNotifier struct{}

func (logNotifier) Send(_ context.Context, m SubscribeMsg) error {
	log.Printf("subscribe msg (not sent): to=%s tmpl=%s data=%v", m.ToUser, m.TemplateID, m.Data)
	return nil
}

// ---------- 消息内容 ----------

// thing 类字段最多 20 个字
func thing(s string) string {
	if utf8.RuneCountInString(s) <= 20 {
		return s
	}
	r := []rune(s)
	return string(r[:19]) + "…"
}

// msgTime 例如“2026年10月01日 14:00”
func msgTime(b *BookingRow) string { return b.StartAt.In(shanghai).Format("2006年01月02日 15:04") }

func vals(kv ...string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = map[string]string{"value": kv[i+1]}
	}
	return out
}

// confirmedMsg 当天才确认的预约不会再有到店提醒，备注里就不说“前一天会再提醒你”
func (a *App) confirmedMsg(openid string, b *BookingRow, shop Shop) SubscribeMsg {
	note := "素颜过来就好，前一天会再提醒你"
	if b.Date <= a.today() {
		note = "素颜过来就好"
	}
	return SubscribeMsg{ToUser: openid, TemplateID: tmplConfirmed, Page: msgPage, Data: vals(
		"thing8", thing(b.ServiceName),
		"date17", msgTime(b),
		"thing14", thing(shop.Name+" · "+b.ArtistName),
		"thing19", note,
	)}
}

func reminderMsg(openid string, b *BookingRow, shop Shop) SubscribeMsg {
	return SubscribeMsg{ToUser: openid, TemplateID: tmplReminder, Page: msgPage, Data: vals(
		"thing1", thing(b.ServiceName),
		"time3", msgTime(b),
		"thing6", thing(shop.Address),
		"thing5", "明天见，素颜过来就好",
	)}
}

// noticeKey 同一条预约、同一个时段、同一种消息只发一次
func noticeKey(b *BookingRow, kind string) string {
	return b.ID + "|" + kind + "|" + b.Date + "|" + b.Time
}

// ---------- 发送 ----------

// notify 先在 notices 表里占位再发，多个实例同时跑也不会重复发。
// 可以重试的错误把占位删掉，下一轮定时任务再发
func (a *App) notify(ctx context.Context, b *BookingRow, kind string, msg func(string, *BookingRow, Shop) SubscribeMsg) {
	if a.notifier == nil {
		return
	}
	key := noticeKey(b, kind)
	ok, err := a.store.ClaimNotice(ctx, key, b.ID, kind, a.now().UTC())
	if err != nil || !ok {
		if err != nil {
			log.Printf("notice %s: %v", key, err)
		}
		return
	}
	users, err := a.store.UsersByIDs(ctx, []string{b.UserID})
	if err != nil {
		_ = a.store.ReleaseNotice(ctx, key)
		log.Printf("notice %s: %v", key, err)
		return
	}
	u := users[b.UserID]
	if u == nil {
		_ = a.store.FinishNotice(ctx, key, "no_user")
		return
	}
	shop, err := a.Shop(ctx)
	if err != nil {
		_ = a.store.ReleaseNotice(ctx, key)
		log.Printf("notice %s: %v", key, err)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = a.notifier.Send(sendCtx, msg(u.OpenID, b, shop))
	switch {
	case err == nil:
		_ = a.store.FinishNotice(ctx, key, "sent")
	case retryable(err):
		_ = a.store.ReleaseNotice(ctx, key)
		log.Printf("notice %s: %v（下一轮重试）", key, err)
	default:
		_ = a.store.FinishNotice(ctx, key, thing(err.Error()))
		log.Printf("notice %s: %v", key, err)
	}
}

// SendReminders 定时任务：给明天的已确认预约发到店提醒。只在前一天 reminderHour 点以后发；
// 错过了（比如当天才确认）就不发，客人已经收到确认消息了
func (a *App) SendReminders(ctx context.Context) error {
	now := a.now().In(shanghai)
	if now.Hour() < reminderHour {
		return nil
	}
	list, err := a.store.ActiveBookingsOn(ctx, dateAfter(a.today(), 1))
	if err != nil {
		return err
	}
	for _, b := range list {
		if b.Status == StatusConfirmed {
			a.notify(ctx, b, noticeReminder, reminderMsg)
		}
	}
	return nil
}

// ---------- 通知店主 ----------
//
// 客人付完定金、改期、取消（只算付过定金的）时，给每位店主发一条“预约变动提醒”。
// 也是一次性订阅：店主在排班页每同意一次，额度加一（AddOwnerNotify），每发一条减一。
// 不经过 notices 表：这几个时机本身只会发生一次（支付回调重复到达时 MarkPaid 不会再走到这里）。

// tmplOwner 新订单提醒（公共模板 28904），和 src/utils/subscribe.ts 的 owner 一致。留空时不发也不扣额度
var tmplOwner = "-wVPwTNjeSYb3YC5OpABnetI94tdELo593BIVXSC0tc"

const (
	ownerKeyService  = "thing44"  // 项目名称
	ownerKeyTime     = "time43"   // 预定日期
	ownerKeyCustomer = "thing17"  // 客人姓名：客人 · 化妆师
	ownerKeyStatus   = "phrase12" // 订单状态：新预约 / 已改期 / 已取消
	ownerKeyNote     = "thing15"  // 订单备注

	ownerMsgPage = "pages-owner/schedule/index"
)

type ownerNotice struct {
	status, note string
}

var (
	ownerNew         = ownerNotice{"新预约", "定金已付，等你确认"}
	ownerRescheduled = ownerNotice{"已改期", "改到这个时间，等你重新确认"}
	ownerCancelled   = ownerNotice{"已取消", "定金已退，这个时段空出来了"}
)

func ownerMsg(openid string, b *BookingRow, customer *User, k ownerNotice) SubscribeMsg {
	return SubscribeMsg{ToUser: openid, TemplateID: tmplOwner, Page: ownerMsgPage + "?date=" + b.Date, Data: vals(
		ownerKeyService, thing(b.ServiceName),
		ownerKeyTime, msgTime(b),
		ownerKeyCustomer, thing(customer.Nickname+" · "+b.ArtistName),
		ownerKeyStatus, k.status,
		ownerKeyNote, k.note,
	)}
}

// notifyOwners 有额度才发；没发出去把额度退回，微信说没订阅（43101 等）就清零，让店主重新续
func (a *App) notifyOwners(ctx context.Context, b *BookingRow, customer *User, k ownerNotice) {
	if a.notifier == nil || tmplOwner == "" {
		return
	}
	for openid := range a.owners {
		ok, err := a.store.TakeOwnerNotifyQuota(ctx, openid)
		if err != nil || !ok {
			if err != nil {
				log.Printf("owner notice %s %s: %v", b.ID, k.status, err)
			}
			continue
		}
		sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = a.notifier.Send(sendCtx, ownerMsg(openid, b, customer, k))
		cancel()
		if err == nil {
			continue
		}
		log.Printf("owner notice %s %s: %v", b.ID, k.status, err)
		var we *WxError
		if errors.As(err, &we) && we.Code == 43101 {
			_, _ = a.store.AddOwnerNotifyQuota(ctx, openid, -math.MaxInt32) // 清零
		} else {
			_, _ = a.store.AddOwnerNotifyQuota(ctx, openid, 1)
		}
	}
}

// OwnerNotify GET /owner/notify
func (a *App) OwnerNotify(ctx context.Context, u *User) (OwnerNotifyStatus, error) {
	if err := a.requireOwner(u); err != nil {
		return OwnerNotifyStatus{}, err
	}
	n, err := a.store.OwnerNotifyQuota(ctx, u.OpenID)
	return OwnerNotifyStatus{Quota: n}, err
}

// AddOwnerNotify POST /owner/notify  店主在小程序里同意了 count 次订阅
func (a *App) AddOwnerNotify(ctx context.Context, u *User, req OwnerNotifyReq) (OwnerNotifyStatus, error) {
	if err := a.requireOwner(u); err != nil {
		return OwnerNotifyStatus{}, err
	}
	if req.Count < 1 || req.Count > 5 {
		return OwnerNotifyStatus{}, apiErr("UNKNOWN", "额度一次只能加 1–5 条")
	}
	n, err := a.store.AddOwnerNotifyQuota(ctx, u.OpenID, req.Count)
	return OwnerNotifyStatus{Quota: n}, err
}
