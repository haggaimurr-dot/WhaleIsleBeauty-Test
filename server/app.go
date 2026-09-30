package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // 镜像里没有时区数据，编进二进制
	"unicode/utf8"
)

// 业务规则，和前端 mock.ts 的行为保持一致。

var shanghai = mustLoadLocation("Asia/Shanghai")

const (
	payWindow     = 15 * time.Minute
	cancelWindow  = 24 * time.Hour
	bookableDays  = 7 // 客人从明天起可以约 7 天
	scheduleDays  = 8 // 店主从今天起看 8 天
	maxTextLength = 200
)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// ---------- 错误 ----------

type ApiError struct {
	Status  int
	Code    string
	Message string
}

func (e *ApiError) Error() string { return e.Code + ": " + e.Message }

var codeStatus = map[string]int{
	"SLOT_TAKEN":      http.StatusConflict,
	"CANCEL_TOO_LATE": http.StatusConflict,
	"INVALID_STATE":   http.StatusConflict,
	"NOT_FOUND":       http.StatusNotFound,
	"FORBIDDEN":       http.StatusForbidden,
	"UNAUTHORIZED":    http.StatusUnauthorized,
	"UNKNOWN":         http.StatusBadRequest,
}

func apiErr(code, msg string) *ApiError {
	return &ApiError{Status: codeStatus[code], Code: code, Message: msg}
}

var (
	errNotFound   = apiErr("NOT_FOUND", "没有找到这条信息")
	errSlotTaken  = apiErr("SLOT_TAKEN", "这个时间刚被约走了，换一个时间吧")
	errForbidden  = apiErr("FORBIDDEN", "只有店主可以进行这个操作")
	errStateMoved = apiErr("INVALID_STATE", "这个预约的状态已经变了")
	errPayTimeout = apiErr("INVALID_STATE", "超过 15 分钟没付定金，这个时段已经放出去了，重新约一次吧")
)

func badRequest(msg string) *ApiError { return apiErr("UNKNOWN", msg) }

// ---------- App ----------

type App struct {
	store   Store
	now     func() time.Time
	owners  map[string]bool // 店主的 openid
	fakePay bool            // 还没有商户号：不调微信支付，由 /fake-paid 模拟回调
	// fakePayers 能调 /fake-paid 的 openid（店主 + 测试人员）。别的客人调会被拒绝，防止不付钱就约上
	fakePayers map[string]bool
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (a *App) today() string { return a.now().In(shanghai).Format("2006-01-02") }

func dateAfter(date string, days int) string {
	d, _ := time.ParseInLocation("2006-01-02", date, shanghai)
	return d.AddDate(0, 0, days).Format("2006-01-02")
}

func (a *App) datesFrom(offset, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = dateAfter(a.today(), offset+i)
	}
	return out
}

func parseStart(date, t string) (time.Time, bool) {
	v, err := time.ParseInLocation("2006-01-02 15:04", date+" "+t, shanghai)
	return v, err == nil
}

func validDate(date string) bool {
	_, err := time.ParseInLocation("2006-01-02", date, shanghai)
	return err == nil && len(date) == 10
}

func (a *App) isPast(date, t string) bool {
	start, _ := parseStart(date, t)
	return !start.After(a.now())
}

func (a *App) bookable(date string) bool {
	for _, d := range a.datesFrom(1, bookableDays) {
		if d == date {
			return true
		}
	}
	return false
}

func ts(t time.Time) string { return t.In(shanghai).Format(time.RFC3339) }

func (a *App) canCancel(b *BookingRow) bool {
	if b.Status == StatusPendingPayment {
		return true
	}
	return isActive(b.Status) && b.StartAt.Sub(a.now()) >= cancelWindow
}

func (a *App) out(b *BookingRow) Booking {
	o := Booking{
		ID: b.ID, ServiceID: b.ServiceID, ServiceName: b.ServiceName,
		ArtistID: b.ArtistID, ArtistName: b.ArtistName, Date: b.Date, Time: b.Time,
		DurationMin: b.DurationMin, Price: b.Price, Deposit: b.Deposit, Status: b.Status,
		Occasion: b.Occasion, SkinType: b.SkinType, Note: b.Note,
		CreatedAt: ts(b.CreatedAt), CanCancel: a.canCancel(b), CancelReason: b.CancelReason,
	}
	if b.Status == StatusPendingPayment && b.PayDeadline != nil {
		o.PayDeadline = ts(*b.PayDeadline)
	}
	return o
}

// ---------- 自动流转（原来设想的定时任务） ----------

// Sweep 把到期的预约流转掉。每个请求前都跑一次，服务缩到 0 个实例时也不会漏；另外起了一个每分钟的定时器
func (a *App) Sweep(ctx context.Context) error {
	now := a.now()
	due, err := a.store.DueBookings(ctx, now)
	if err != nil {
		return err
	}
	for _, b := range due {
		from := b.Status
		switch from {
		case StatusPendingPayment:
			b.Status, b.CancelReason, b.PayDeadline = StatusCancelled, ReasonPayTimeout, nil
		case StatusPendingConfirm:
			b.Status, b.CancelReason = StatusCancelled, ReasonNotConfirmed
			a.refund(b, now)
		case StatusConfirmed:
			// 契约里没有“到店完成”的接口：已确认的预约过了结束时间就算完成
			b.Status = StatusCompleted
		}
		if err := a.store.UpdateBooking(ctx, b, from); err != nil && !errors.Is(err, ErrStale) {
			return err
		}
	}
	return nil
}

// refund 定金原路退回。假支付模式下只记下退款时间
func (a *App) refund(b *BookingRow, now time.Time) {
	if b.PaidAt == nil || b.RefundedAt != nil {
		return
	}
	if !a.fakePay {
		// TODO: 接微信支付后在这里调退款接口
		log.Printf("refund needed: booking=%s deposit=%d", b.ID, b.Deposit)
		return
	}
	b.RefundedAt = &now
}

// ---------- 用户 ----------

func (a *App) UserFor(ctx context.Context, openid string) (*User, error) {
	u, err := a.store.UserByOpenID(ctx, openid)
	if err != nil || u != nil {
		return u, err
	}
	id := newID("u")
	// 微信已不再给昵称，先用 id 尾号区分客人，以后可以让客人自己改
	return a.store.CreateUser(ctx, &User{
		ID: id, OpenID: openid, Nickname: "客人" + id[len(id)-4:], Avatar: "placeholder:g4", CreatedAt: a.now().UTC(),
	})
}

var skinSummaryLabel = map[string]string{"dry": "干皮", "oily": "油皮", "combination": "混合皮", "sensitive": "敏感肌"}
var toneLabel = map[string]string{"cool_fair": "冷白皮", "warm_fair": "暖白皮", "natural": "自然色", "wheat": "小麦色"}

func hasProfile(p SkinProfile) bool {
	return p.SkinType != "" || p.Tone != "" || p.Allergies != "" || p.Note != ""
}

// skinSummary 例如“敏感肌 · 冷白皮”；只填了文字说明时写“已填写”
func skinSummary(p SkinProfile) string {
	var parts []string
	if l := skinSummaryLabel[p.SkinType]; l != "" {
		parts = append(parts, l)
	}
	if l := toneLabel[p.Tone]; l != "" {
		parts = append(parts, l)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " · ")
	}
	if hasProfile(p) {
		return "已填写"
	}
	return ""
}

func (a *App) Me(ctx context.Context, u *User) (Me, error) {
	n, err := a.store.CountCompleted(ctx, u.ID)
	if err != nil {
		return Me{}, err
	}
	role := "customer"
	if a.owners[u.OpenID] {
		role = "owner"
	}
	return Me{ID: u.ID, Nickname: u.Nickname, Avatar: u.Avatar, Role: role, VisitCount: n, SkinProfile: skinSummary(u.Skin)}, nil
}

func (a *App) UpdateSkinProfile(ctx context.Context, u *User, req SkinProfile) (SkinProfile, error) {
	if req.SkinType != "" && !skinTypes[req.SkinType] || req.Tone != "" && !skinTones[req.Tone] {
		return SkinProfile{}, badRequest("肤质选项不对")
	}
	p := SkinProfile{
		SkinType: req.SkinType, Tone: req.Tone,
		Allergies: strings.TrimSpace(req.Allergies), Note: strings.TrimSpace(req.Note),
		UpdatedAt: ts(a.now()),
	}
	if utf8.RuneCountInString(p.Allergies) > maxTextLength || utf8.RuneCountInString(p.Note) > maxTextLength {
		return SkinProfile{}, badRequest("写得有点多了，控制在 200 字以内")
	}
	if err := a.store.SaveSkinProfile(ctx, u.ID, p); err != nil {
		return SkinProfile{}, err
	}
	return p, nil
}

// ---------- 客人端预约 ----------

func (a *App) ListSlots(ctx context.Context, u *User, artistID, date, excludeID string) ([]SlotView, error) {
	if findArtist(artistID) == nil {
		return nil, errNotFound
	}
	if !validDate(date) {
		return nil, badRequest("日期格式不对")
	}
	// 可约日期以外的日子全部显示约满，和下单时的检查一致
	if !a.bookable(date) {
		out := make([]SlotView, len(slotTimes))
		for i, t := range slotTimes {
			out[i] = SlotView{Time: t}
		}
		return out, nil
	}
	// 改期时正在改的那条：必须是自己的、还在进行中，否则忽略
	if excludeID != "" {
		b, err := a.store.Booking(ctx, excludeID)
		if err != nil || b.UserID != u.ID || !isActive(b.Status) {
			excludeID = ""
		}
	}
	taken, err := a.takenOn(ctx, date)
	if err != nil {
		return nil, err
	}
	blocks, err := a.store.BlocksOn(ctx, date)
	if err != nil {
		return nil, err
	}
	out := make([]SlotView, len(slotTimes))
	for i, t := range slotTimes {
		holder := taken[artistID+"|"+t]
		free := holder == nil || holder.ID == excludeID
		out[i] = SlotView{Time: t, Available: free && !blocks[artistID+"|"+t] && !a.isPast(date, t)}
	}
	return out, nil
}

// takenOn 这一天被进行中的预约占着的时段，key 是 artistID|time
func (a *App) takenOn(ctx context.Context, date string) (map[string]*BookingRow, error) {
	list, err := a.store.ActiveBookingsOn(ctx, date)
	if err != nil {
		return nil, err
	}
	m := map[string]*BookingRow{}
	for _, b := range list {
		m[b.ArtistID+"|"+b.Time] = b
	}
	return m, nil
}

// checkSlot 客人下单或改期时，这个时段能不能约
func (a *App) checkSlot(ctx context.Context, artistID, date, t string) (*Artist, error) {
	artist := findArtist(artistID)
	if artist == nil {
		return nil, errNotFound
	}
	if !validDate(date) || !isSlotTime(t) {
		return nil, badRequest("时间格式不对")
	}
	if !a.bookable(date) {
		return nil, apiErr("INVALID_STATE", "这一天现在还不能约，换一天吧")
	}
	if a.isPast(date, t) {
		return nil, errSlotTaken
	}
	blocks, err := a.store.BlocksOn(ctx, date)
	if err != nil {
		return nil, err
	}
	if blocks[artistID+"|"+t] {
		return nil, errSlotTaken
	}
	return artist, nil
}

func (a *App) payParams(b *BookingRow) WxPayParams {
	// TODO: 拿到商户号后，走云托管开放接口调微信支付 V3 下单
	return WxPayParams{
		TimeStamp: fmt.Sprint(a.now().Unix()), NonceStr: "fake",
		Package: "prepay_id=fake_" + b.ID, SignType: "RSA", PaySign: "fake",
	}
}

func (a *App) CreateBooking(ctx context.Context, u *User, req CreateBookingReq) (CreateBookingResp, error) {
	svc := findService(req.ServiceID)
	if svc == nil {
		return CreateBookingResp{}, errNotFound
	}
	if req.Occasion != "" && !occasions[req.Occasion] || req.SkinType != "" && !skinTypes[req.SkinType] {
		return CreateBookingResp{}, badRequest("选项不对")
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxTextLength {
		return CreateBookingResp{}, badRequest("备注写得有点多了，控制在 200 字以内")
	}
	artist, err := a.checkSlot(ctx, req.ArtistID, req.Date, req.Time)
	if err != nil {
		return CreateBookingResp{}, err
	}
	now := a.now().UTC()
	start, _ := parseStart(req.Date, req.Time)
	deadline := now.Add(payWindow)
	b := &BookingRow{
		ID: newID("b"), UserID: u.ID,
		ServiceID: svc.ID, ServiceName: svc.Name, ArtistID: artist.ID, ArtistName: artist.Name,
		Date: req.Date, Time: req.Time, StartAt: start.UTC(), EndAt: start.Add(time.Duration(svc.DurationMin) * time.Minute).UTC(),
		DurationMin: svc.DurationMin, Price: svc.Price, Deposit: svc.Deposit,
		Status: StatusPendingPayment, Occasion: req.Occasion, SkinType: req.SkinType, Note: note,
		CreatedAt: now, PayDeadline: &deadline,
	}
	if err := a.store.InsertBooking(ctx, b); err != nil {
		if errors.Is(err, ErrSlotTaken) {
			return CreateBookingResp{}, errSlotTaken
		}
		return CreateBookingResp{}, err
	}
	return CreateBookingResp{Booking: a.out(b), Payment: a.payParams(b)}, nil
}

// myBooking 只能看自己的预约；别人的按不存在处理
func (a *App) myBooking(ctx context.Context, u *User, id string) (*BookingRow, error) {
	b, err := a.store.Booking(ctx, id)
	if errors.Is(err, ErrNotFound) || err == nil && b.UserID != u.ID {
		return nil, errNotFound
	}
	return b, err
}

func (a *App) GetBooking(ctx context.Context, u *User, id string) (Booking, error) {
	b, err := a.myBooking(ctx, u, id)
	if err != nil {
		return Booking{}, err
	}
	return a.out(b), nil
}

func (a *App) ListMyBookings(ctx context.Context, u *User, scope string) ([]Booking, error) {
	upcoming := scope == "upcoming"
	statuses := pastStatuses
	if upcoming {
		statuses = activeStatuses
	}
	list, err := a.store.BookingsByUser(ctx, u.ID, statuses)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(list, func(i, j int) bool {
		if upcoming {
			return list[i].StartAt.Before(list[j].StartAt)
		}
		return list[i].StartAt.After(list[j].StartAt)
	})
	out := make([]Booking, len(list))
	for i, b := range list {
		out[i] = a.out(b)
	}
	return out, nil
}

func (a *App) ResumePayment(ctx context.Context, u *User, id string) (CreateBookingResp, error) {
	b, err := a.myBooking(ctx, u, id)
	if err != nil {
		return CreateBookingResp{}, err
	}
	switch {
	case b.Status == StatusCancelled && b.CancelReason == ReasonPayTimeout:
		return CreateBookingResp{}, errPayTimeout
	case b.Status != StatusPendingPayment:
		return CreateBookingResp{}, apiErr("INVALID_STATE", "这个预约已经付过定金了")
	}
	return CreateBookingResp{Booking: a.out(b), Payment: a.payParams(b)}, nil
}

// FakePaid 假支付：只有店主和测试人员能用，其他客人返回 FORBIDDEN
func (a *App) FakePaid(ctx context.Context, u *User, id string) (Booking, error) {
	if !a.fakePay || !a.fakePayers[u.OpenID] {
		return Booking{}, apiErr("FORBIDDEN", "现在还不能在线付定金，请联系门店预约")
	}
	return a.MarkPaid(ctx, u, id)
}

// MarkPaid 支付回调：待付定金 → 等待店里确认。假支付模式下由 FakePaid 调用
func (a *App) MarkPaid(ctx context.Context, u *User, id string) (Booking, error) {
	b, err := a.myBooking(ctx, u, id)
	if err != nil {
		return Booking{}, err
	}
	if b.Status != StatusPendingPayment {
		// 回调可能重复到达，已经处理过就直接返回
		if b.PaidAt != nil {
			return a.out(b), nil
		}
		return Booking{}, errPayTimeout
	}
	now := a.now().UTC()
	b.Status, b.PayDeadline, b.PaidAt = StatusPendingConfirm, nil, &now
	if err := a.store.UpdateBooking(ctx, b, StatusPendingPayment); err != nil {
		if errors.Is(err, ErrStale) {
			return Booking{}, errStateMoved
		}
		return Booking{}, err
	}
	return a.out(b), nil
}

// changeable 取消和改期共用的检查
func (a *App) changeable(b *BookingRow, verb string) error {
	if !isActive(b.Status) {
		return errStateMoved
	}
	if !a.canCancel(b) {
		return apiErr("CANCEL_TOO_LATE", "距离开始不到 24 小时，需要"+verb+"请直接联系门店")
	}
	return nil
}

func (a *App) CancelBooking(ctx context.Context, u *User, id string) (Booking, error) {
	b, err := a.myBooking(ctx, u, id)
	if err != nil {
		return Booking{}, err
	}
	if err := a.changeable(b, "取消"); err != nil {
		return Booking{}, err
	}
	from := b.Status
	b.Status, b.CancelReason, b.PayDeadline = StatusCancelled, ReasonCustomer, nil
	a.refund(b, a.now().UTC())
	if err := a.store.UpdateBooking(ctx, b, from); err != nil {
		if errors.Is(err, ErrStale) {
			return Booking{}, errStateMoved
		}
		return Booking{}, err
	}
	return a.out(b), nil
}

func (a *App) RescheduleBooking(ctx context.Context, u *User, id string, req SlotReq) (Booking, error) {
	b, err := a.myBooking(ctx, u, id)
	if err != nil {
		return Booking{}, err
	}
	if err := a.changeable(b, "改期"); err != nil {
		return Booking{}, err
	}
	artist, err := a.checkSlot(ctx, req.ArtistID, req.Date, req.Time)
	if err != nil {
		return Booking{}, err
	}
	from := b.Status
	start, _ := parseStart(req.Date, req.Time)
	b.ArtistID, b.ArtistName, b.Date, b.Time = artist.ID, artist.Name, req.Date, req.Time
	b.StartAt, b.EndAt = start.UTC(), start.Add(time.Duration(b.DurationMin)*time.Minute).UTC()
	// 按契约改期后回到 pending_confirm；还没付定金的仍然是待付定金
	if from != StatusPendingPayment {
		b.Status = StatusPendingConfirm
	}
	if err := a.store.UpdateBooking(ctx, b, from); err != nil {
		switch {
		case errors.Is(err, ErrSlotTaken):
			return Booking{}, errSlotTaken
		case errors.Is(err, ErrStale):
			return Booking{}, errStateMoved
		}
		return Booking{}, err
	}
	return a.out(b), nil
}

// ---------- 店主端 ----------

func (a *App) requireOwner(u *User) error {
	if !a.owners[u.OpenID] {
		return errForbidden
	}
	return nil
}

func (a *App) ScheduleDates(u *User) ([]string, error) {
	if err := a.requireOwner(u); err != nil {
		return nil, err
	}
	return a.datesFrom(0, scheduleDays), nil
}

// ownerAlert 合并这次预约和客人档案里需要化妆师提前知道的
func ownerAlert(b *BookingRow, p SkinProfile) (alert, note string) {
	sensitive := b.SkinType == "sensitive" || p.SkinType == "sensitive"
	var notes []string
	if b.Note != "" {
		notes = append(notes, b.Note)
	}
	if p.Allergies != "" {
		notes = append(notes, "过敏："+p.Allergies)
	}
	switch {
	case sensitive:
		alert = "敏感肌"
	case p.Allergies != "":
		alert = "有过敏"
	}
	return alert, strings.Join(notes, "；")
}

func (a *App) DaySchedule(ctx context.Context, u *User, date string) (DaySchedule, error) {
	if err := a.requireOwner(u); err != nil {
		return DaySchedule{}, err
	}
	if !validDate(date) {
		return DaySchedule{}, badRequest("日期格式不对")
	}
	taken, err := a.takenOn(ctx, date)
	if err != nil {
		return DaySchedule{}, err
	}
	blocks, err := a.store.BlocksOn(ctx, date)
	if err != nil {
		return DaySchedule{}, err
	}
	var userIDs []string
	for _, b := range taken {
		userIDs = append(userIDs, b.UserID)
	}
	users, err := a.store.UsersByIDs(ctx, userIDs)
	if err != nil {
		return DaySchedule{}, err
	}

	ds := DaySchedule{Date: date, Times: slotTimes, Cells: []ScheduleCell{}}
	for _, ar := range artists {
		ds.Artists = append(ds.Artists, ArtistBrief{ID: ar.ID, Name: ar.Name})
	}
	for _, t := range slotTimes {
		for _, ar := range artists {
			cell := ScheduleCell{ArtistID: ar.ID, Time: t, State: "free"}
			key := ar.ID + "|" + t
			if b := taken[key]; b != nil {
				customer := users[b.UserID]
				if customer == nil {
					customer = &User{Nickname: "客人"}
				}
				alert, note := ownerAlert(b, customer.Skin)
				brief := &OwnerBookingBrief{
					ID: b.ID, CustomerName: customer.Nickname, ServiceName: b.ServiceName, Status: b.Status,
					Alert: alert, Note: note,
					CanConfirm:  b.Status == StatusPendingConfirm && !a.isPast(b.Date, b.Time),
					DurationMin: b.DurationMin, Occasion: b.Occasion, SkinType: b.SkinType,
				}
				if hasProfile(customer.Skin) {
					p := customer.Skin
					brief.Profile = &p
				}
				// 待付定金也算占着：付款截止前为客人保留
				cell.State = "booked"
				if b.Status == StatusPendingConfirm {
					cell.State = "pending"
				}
				cell.Booking = brief
			} else if a.isPast(date, t) {
				cell.State = "past"
			} else if blocks[key] {
				cell.State = "blocked"
			}
			unpaid := cell.Booking != nil && cell.Booking.Status == StatusPendingPayment
			if (cell.State == "booked" || cell.State == "pending") && !unpaid {
				ds.Stats.Total++
			}
			if cell.State == "pending" {
				ds.Stats.Pending++
				if !cell.Booking.CanConfirm {
					ds.Stats.Stale++
				}
			}
			if cell.State == "free" {
				ds.Stats.Free++
			}
			ds.Cells = append(ds.Cells, cell)
		}
	}
	return ds, nil
}

func (a *App) ConfirmBooking(ctx context.Context, u *User, id string) (Booking, error) {
	if err := a.requireOwner(u); err != nil {
		return Booking{}, err
	}
	b, err := a.store.Booking(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Booking{}, errNotFound
	} else if err != nil {
		return Booking{}, err
	}
	if b.Status != StatusPendingConfirm {
		return Booking{}, apiErr("INVALID_STATE", "这个预约已经处理过了")
	}
	if a.isPast(b.Date, b.Time) {
		return Booking{}, apiErr("INVALID_STATE", "预约时间已经过了，不能再确认")
	}
	b.Status = StatusConfirmed
	if err := a.store.UpdateBooking(ctx, b, StatusPendingConfirm); err != nil {
		if errors.Is(err, ErrStale) {
			return Booking{}, apiErr("INVALID_STATE", "这个预约已经处理过了")
		}
		return Booking{}, err
	}
	// TODO: 发“预约确认”订阅消息
	return a.out(b), nil
}

func (a *App) checkOwnerSlot(u *User, req SlotReq) error {
	if err := a.requireOwner(u); err != nil {
		return err
	}
	if findArtist(req.ArtistID) == nil {
		return errNotFound
	}
	if !validDate(req.Date) || !isSlotTime(req.Time) {
		return badRequest("时间格式不对")
	}
	if a.isPast(req.Date, req.Time) {
		return apiErr("INVALID_STATE", "这个时段已经过去了")
	}
	return nil
}

func (a *App) BlockSlot(ctx context.Context, u *User, req SlotReq) error {
	if err := a.checkOwnerSlot(u, req); err != nil {
		return err
	}
	taken, err := a.takenOn(ctx, req.Date)
	if err != nil {
		return err
	}
	if taken[req.ArtistID+"|"+req.Time] != nil {
		return apiErr("INVALID_STATE", "这个时段已经有预约了")
	}
	return a.store.AddBlock(ctx, req.ArtistID, req.Date, req.Time)
}

func (a *App) UnblockSlot(ctx context.Context, u *User, req SlotReq) error {
	if err := a.checkOwnerSlot(u, req); err != nil {
		return err
	}
	return a.store.RemoveBlock(ctx, req.ArtistID, req.Date, req.Time)
}
