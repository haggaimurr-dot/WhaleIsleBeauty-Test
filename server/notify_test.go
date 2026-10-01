package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeNotifier 记下发出的消息；fail 不为空时依次返回里面的错误
type fakeNotifier struct {
	mu   sync.Mutex
	sent []SubscribeMsg
	fail []error
}

func (n *fakeNotifier) Send(_ context.Context, m SubscribeMsg) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.fail) > 0 {
		err := n.fail[0]
		n.fail = n.fail[1:]
		if err != nil {
			return err
		}
	}
	n.sent = append(n.sent, m)
	return nil
}

func (n *fakeNotifier) take() []SubscribeMsg {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := n.sent
	n.sent = nil
	return out
}

func notifyApp(t *testing.T) (*App, *clock, *fakeNotifier) {
	a, c := newTestApp(t)
	n := &fakeNotifier{}
	a.notifier = n
	return a, c, n
}

func confirm(t *testing.T, a *App, id string) {
	t.Helper()
	if _, err := a.ConfirmBooking(context.Background(), user(t, a, ownerOpenID), id); err != nil {
		t.Fatal(err)
	}
}

func val(m SubscribeMsg, key string) string { return m.Data[key]["value"] }

func TestConfirmSendsMessage(t *testing.T) {
	a, _, n := notifyApp(t)
	ctx := context.Background()
	alice := user(t, a, "o-alice")
	b := paid(t, a, alice, "2026-10-03", "13:30")
	if got := n.take(); len(got) != 0 {
		t.Fatalf("付定金不发消息，发了 %v", got)
	}
	confirm(t, a, b.ID)
	got := n.take()
	if len(got) != 1 {
		t.Fatalf("want 1 message, got %d", len(got))
	}
	m := got[0]
	if m.ToUser != "o-alice" || m.TemplateID != tmplConfirmed || m.Page != msgPage {
		t.Fatalf("unexpected %+v", m)
	}
	if val(m, "thing8") != "韩式上镜妆（含发型）" || val(m, "date17") != "2026年10月03日 13:30" ||
		val(m, "thing14") != "鲸屿美妆 · 小鲸" || val(m, "thing19") != "素颜过来就好，前一天会再提醒你" {
		t.Fatalf("unexpected data %v", m.Data)
	}

	// 改期后店主再确认，按新时间再发一次
	if _, err := a.RescheduleBooking(ctx, alice, b.ID, SlotReq{ArtistID: "a2", Date: "2026-10-04", Time: "09:00"}); err != nil {
		t.Fatal(err)
	}
	confirm(t, a, b.ID)
	got = n.take()
	if len(got) != 1 || val(got[0], "date17") != "2026年10月04日 09:00" || val(got[0], "thing14") != "鲸屿美妆 · 安安" {
		t.Fatalf("改期后应按新时间发，got %v", got)
	}
}

func TestConfirmSameDayNote(t *testing.T) {
	a, c, n := notifyApp(t)
	b := paid(t, a, user(t, a, "o-alice"), "2026-10-02", "19:30")
	c.advance(24 * time.Hour) // 10-02 10:00，当天才确认
	confirm(t, a, b.ID)
	if got := n.take(); len(got) != 1 || val(got[0], "thing19") != "素颜过来就好" {
		t.Fatalf("当天确认不该说前一天提醒，got %v", got)
	}
}

func TestReminders(t *testing.T) {
	a, c, n := notifyApp(t)
	ctx := context.Background()
	alice, bob := user(t, a, "o-alice"), user(t, a, "o-bob")
	confirmed := paid(t, a, alice, "2026-10-02", "10:30")
	confirm(t, a, confirmed.ID)
	paid(t, a, bob, "2026-10-02", "12:00")          // 还没确认的不提醒
	later := paid(t, a, bob, "2026-10-03", "12:00") // 后天的今天不提醒
	confirm(t, a, later.ID)
	n.take()

	if err := a.SendReminders(ctx); err != nil {
		t.Fatal(err)
	}
	if got := n.take(); len(got) != 0 {
		t.Fatalf("18 点前不发，got %v", got)
	}

	c.advance(8 * time.Hour) // 10-01 18:00
	_ = a.SendReminders(ctx)
	got := n.take()
	if len(got) != 1 {
		t.Fatalf("want 1 reminder, got %v", got)
	}
	m := got[0]
	if m.ToUser != "o-alice" || m.TemplateID != tmplReminder || val(m, "thing1") != "韩式上镜妆（含发型）" ||
		val(m, "time3") != "2026年10月02日 10:30" || val(m, "thing6") != seedShop.Address || val(m, "thing5") != "明天见，素颜过来就好" {
		t.Fatalf("unexpected %+v", m)
	}

	// 每分钟都会跑，同一条只发一次
	c.advance(time.Minute)
	_ = a.SendReminders(ctx)
	if got := n.take(); len(got) != 0 {
		t.Fatalf("重复发了 %v", got)
	}
}

func TestReminderRetry(t *testing.T) {
	a, c, n := notifyApp(t)
	ctx := context.Background()
	b1 := paid(t, a, user(t, a, "o-alice"), "2026-10-02", "10:30")
	b2 := paid(t, a, user(t, a, "o-bob"), "2026-10-02", "12:00")
	confirm(t, a, b1.ID)
	confirm(t, a, b2.ID)
	n.take()
	c.advance(8 * time.Hour)

	// 第一条网络出错（下次重试），第二条客人没订阅（不再重试）
	n.fail = []error{io.ErrUnexpectedEOF, &WxError{Code: 43101, Msg: "user refuse to accept the msg"}}
	_ = a.SendReminders(ctx)
	if got := n.take(); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	c.advance(time.Minute)
	_ = a.SendReminders(ctx)
	got := n.take()
	if len(got) != 1 || got[0].ToUser != "o-alice" {
		t.Fatalf("只该重试网络出错的那条，got %v", got)
	}
}

func TestNotifyFailureDoesNotBlockConfirm(t *testing.T) {
	a, _, n := notifyApp(t)
	n.fail = []error{&WxError{Code: 43101}}
	b := paid(t, a, user(t, a, "o-alice"), "2026-10-03", "10:30")
	confirm(t, a, b.ID)
	got, _ := a.GetBooking(context.Background(), user(t, a, "o-alice"), b.ID)
	if got.Status != StatusConfirmed {
		t.Fatalf("消息发不出去也要确认成功，got %s", got.Status)
	}
}

func TestThing(t *testing.T) {
	if thing("短") != "短" {
		t.Fatal("短的不截")
	}
	long := strings.Repeat("字", 25)
	if got := thing(long); len([]rune(got)) != 20 || !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
}

func TestWxNotifier(t *testing.T) {
	var body map[string]any
	var path string
	errcode := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": errcode, "errmsg": "x"})
	}))
	defer srv.Close()
	n := &wxNotifier{base: srv.URL, state: "trial", client: srv.Client()}

	if err := n.Send(context.Background(), SubscribeMsg{ToUser: "o1", TemplateID: "t1", Data: vals("thing1", "a")}); err != nil {
		t.Fatal(err)
	}
	if path != "/cgi-bin/message/subscribe/send" || body["touser"] != "o1" || body["miniprogram_state"] != "trial" {
		t.Fatalf("path=%s body=%v", path, body)
	}
	if v := body["data"].(map[string]any)["thing1"].(map[string]any)["value"]; v != "a" {
		t.Fatalf("data=%v", body["data"])
	}

	errcode = 43101
	err := n.Send(context.Background(), SubscribeMsg{ToUser: "o1"})
	if err == nil || retryable(err) {
		t.Fatalf("43101 不该重试，got %v", err)
	}
	errcode = -1
	if err := n.Send(context.Background(), SubscribeMsg{ToUser: "o1"}); !retryable(err) {
		t.Fatalf("系统繁忙要重试，got %v", err)
	}
}

func TestCronEndpoint(t *testing.T) {
	a, c, n := notifyApp(t)
	b := paid(t, a, user(t, a, "o-alice"), "2026-10-02", "10:30")
	confirm(t, a, b.ID)
	n.take()
	c.advance(8 * time.Hour)

	if code, _, _ := do(t, a.Routes(), "POST", "/cron/reminders", "", ""); code != http.StatusNotFound {
		t.Fatalf("没设口令时不开放，got %d", code)
	}
	a.cronToken = "secret"
	h := a.Routes()
	req := httptest.NewRequest("POST", "/cron/reminders", nil)
	req.Header.Set("X-Cron-Token", "wrong")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("口令不对 got %d", w.Code)
	}
	req.Header.Set("X-Cron-Token", "secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || len(n.take()) != 1 {
		t.Fatalf("got %d", w.Code)
	}
}
