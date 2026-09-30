package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 测试用：内存存储 + 可以拨动的时钟。当前时间固定在 2026-10-01 10:00（上海）

type clock struct{ t time.Time }

func (c *clock) now() time.Time           { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

const ownerOpenID = "o-owner"

func newTestApp(t *testing.T) (*App, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 10, 1, 10, 0, 0, 0, shanghai)}
	return &App{store: newMemStore(), now: c.now, owners: map[string]bool{ownerOpenID: true}, fakePay: true}, c
}

func user(t *testing.T, a *App, openid string) *User {
	t.Helper()
	u, err := a.UserFor(context.Background(), openid)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ae *ApiError
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func book(t *testing.T, a *App, u *User, date, tm string) Booking {
	t.Helper()
	resp, err := a.CreateBooking(context.Background(), u, CreateBookingReq{ServiceID: "s1", ArtistID: "a1", Date: date, Time: tm})
	if err != nil {
		t.Fatal(err)
	}
	return resp.Booking
}

func paid(t *testing.T, a *App, u *User, date, tm string) Booking {
	t.Helper()
	b, err := a.MarkPaid(context.Background(), u, book(t, a, u, date, tm).ID)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSameSlotCannotBeBookedTwice(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	alice, bob := user(t, a, "o-alice"), user(t, a, "o-bob")

	b := book(t, a, alice, "2026-10-03", "10:30")
	if b.Status != StatusPendingPayment || !b.CanCancel || b.PayDeadline == "" {
		t.Fatalf("unexpected booking %+v", b)
	}
	_, err := a.CreateBooking(ctx, bob, CreateBookingReq{ServiceID: "s1", ArtistID: "a1", Date: "2026-10-03", Time: "10:30"})
	wantCode(t, err, "SLOT_TAKEN")

	// 客人端看得到时段被占
	slots, _ := a.ListSlots(ctx, bob, "a1", "2026-10-03", "")
	for _, s := range slots {
		if s.Time == "10:30" && s.Available {
			t.Fatal("10:30 should be taken")
		}
	}
	// 取消后放出来
	if _, err := a.CancelBooking(ctx, alice, b.ID); err != nil {
		t.Fatal(err)
	}
	book(t, a, bob, "2026-10-03", "10:30")
}

func TestOnlyBookableDates(t *testing.T) {
	a, _ := newTestApp(t)
	u := user(t, a, "o-alice")
	for _, d := range []string{"2026-10-01", "2026-10-09"} { // 今天、第 8 天
		_, err := a.CreateBooking(context.Background(), u, CreateBookingReq{ServiceID: "s1", ArtistID: "a1", Date: d, Time: "10:30"})
		wantCode(t, err, "INVALID_STATE")
	}
	book(t, a, u, "2026-10-08", "10:30")
}

func TestPayTimeoutReleasesSlot(t *testing.T) {
	a, c := newTestApp(t)
	ctx := context.Background()
	u := user(t, a, "o-alice")
	b := book(t, a, u, "2026-10-03", "10:30")

	c.advance(15 * time.Minute)
	if err := a.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := a.GetBooking(ctx, u, b.ID)
	if got.Status != StatusCancelled || got.CancelReason != ReasonPayTimeout || got.PayDeadline != "" {
		t.Fatalf("want pay_timeout, got %+v", got)
	}
	_, err := a.ResumePayment(ctx, u, b.ID)
	wantCode(t, err, "INVALID_STATE")
	_, err = a.MarkPaid(ctx, u, b.ID)
	wantCode(t, err, "INVALID_STATE")
	book(t, a, user(t, a, "o-bob"), "2026-10-03", "10:30")
}

func TestCancelWithin24Hours(t *testing.T) {
	a, c := newTestApp(t)
	ctx := context.Background()
	u := user(t, a, "o-alice")

	// 没付定金的随时可以取消
	unpaid := book(t, a, u, "2026-10-02", "09:00")
	// 付了定金的，开始前 24 小时以内不行
	b := paid(t, a, u, "2026-10-03", "09:00")
	c.advance(23*time.Hour + time.Minute) // 距离开始 23 小时不到
	got, _ := a.GetBooking(ctx, u, b.ID)
	if got.CanCancel {
		t.Fatal("canCancel should be false within 24h")
	}
	_, err := a.CancelBooking(ctx, u, b.ID)
	wantCode(t, err, "CANCEL_TOO_LATE")
	_, err = a.RescheduleBooking(ctx, u, b.ID, SlotReq{ArtistID: "a2", Date: "2026-10-05", Time: "09:00"})
	wantCode(t, err, "CANCEL_TOO_LATE")

	c.t = time.Date(2026, 10, 1, 10, 5, 0, 0, shanghai) // 付款窗口内
	if _, err := a.CancelBooking(ctx, u, unpaid.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCancelPaidRefunds(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	u := user(t, a, "o-alice")
	b := paid(t, a, u, "2026-10-05", "09:00")
	if _, err := a.CancelBooking(ctx, u, b.ID); err != nil {
		t.Fatal(err)
	}
	row, _ := a.store.Booking(ctx, b.ID)
	if row.RefundedAt == nil || row.CancelReason != ReasonCustomer {
		t.Fatalf("want refunded customer cancel, got %+v", row)
	}
}

func TestSweepNotConfirmedAndCompleted(t *testing.T) {
	a, c := newTestApp(t)
	ctx := context.Background()
	u, owner := user(t, a, "o-alice"), user(t, a, ownerOpenID)
	unconfirmed := paid(t, a, u, "2026-10-02", "09:00")
	confirmed := paid(t, a, u, "2026-10-02", "10:30")
	if _, err := a.ConfirmBooking(ctx, owner, confirmed.ID); err != nil {
		t.Fatal(err)
	}

	c.t = time.Date(2026, 10, 2, 9, 0, 0, 0, shanghai)
	_ = a.Sweep(ctx)
	got, _ := a.GetBooking(ctx, u, unconfirmed.ID)
	if got.Status != StatusCancelled || got.CancelReason != ReasonNotConfirmed {
		t.Fatalf("want not_confirmed, got %+v", got)
	}
	row, _ := a.store.Booking(ctx, unconfirmed.ID)
	if row.RefundedAt == nil {
		t.Fatal("not_confirmed should refund")
	}

	// 10:30 开始 90 分钟，12:00 结束
	c.t = time.Date(2026, 10, 2, 11, 59, 0, 0, shanghai)
	_ = a.Sweep(ctx)
	if got, _ := a.GetBooking(ctx, u, confirmed.ID); got.Status != StatusConfirmed {
		t.Fatalf("still in progress, got %s", got.Status)
	}
	c.t = time.Date(2026, 10, 2, 12, 0, 0, 0, shanghai)
	_ = a.Sweep(ctx)
	me, _ := a.Me(ctx, u)
	if got, _ := a.GetBooking(ctx, u, confirmed.ID); got.Status != StatusCompleted || me.VisitCount != 1 {
		t.Fatalf("want completed and visitCount 1, got %s %d", got.Status, me.VisitCount)
	}
}

func TestReschedule(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	alice, bob := user(t, a, "o-alice"), user(t, a, "o-bob")
	b := paid(t, a, alice, "2026-10-03", "10:30")
	book(t, a, bob, "2026-10-04", "10:30")

	// 自己占着的时段在改期时显示可约；别人的不行
	slots, _ := a.ListSlots(ctx, alice, "a1", "2026-10-03", b.ID)
	if !slots[1].Available {
		t.Fatal("own slot should be available when rescheduling")
	}
	slots, _ = a.ListSlots(ctx, bob, "a1", "2026-10-03", b.ID)
	if slots[1].Available {
		t.Fatal("excludeBookingId of someone else must be ignored")
	}
	_, err := a.RescheduleBooking(ctx, alice, b.ID, SlotReq{ArtistID: "a1", Date: "2026-10-04", Time: "10:30"})
	wantCode(t, err, "SLOT_TAKEN")

	// 原地改（同一时段）也能成功
	if _, err := a.RescheduleBooking(ctx, alice, b.ID, SlotReq{ArtistID: "a1", Date: "2026-10-03", Time: "10:30"}); err != nil {
		t.Fatal(err)
	}
	got, err := a.RescheduleBooking(ctx, alice, b.ID, SlotReq{ArtistID: "a2", Date: "2026-10-05", Time: "15:00"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusPendingConfirm || got.ArtistName != "安安" || got.Date != "2026-10-05" {
		t.Fatalf("unexpected %+v", got)
	}
	// 原时段放出来了
	book(t, a, bob, "2026-10-03", "10:30")
}

func TestOthersBookingsAreHidden(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	b := book(t, a, user(t, a, "o-alice"), "2026-10-03", "10:30")
	bob := user(t, a, "o-bob")
	_, err := a.GetBooking(ctx, bob, b.ID)
	wantCode(t, err, "NOT_FOUND")
	_, err = a.CancelBooking(ctx, bob, b.ID)
	wantCode(t, err, "NOT_FOUND")
}

func TestOwnerSchedule(t *testing.T) {
	a, c := newTestApp(t)
	ctx := context.Background()
	alice, owner := user(t, a, "o-alice"), user(t, a, ownerOpenID)

	_, err := a.DaySchedule(ctx, alice, "2026-10-02")
	wantCode(t, err, "FORBIDDEN")

	if _, err := a.UpdateSkinProfile(ctx, alice, SkinProfile{Tone: "cool_fair", Allergies: " 对酒精过敏 "}); err != nil {
		t.Fatal(err)
	}
	unpaid := book(t, a, alice, "2026-10-02", "09:00")
	pending := paid(t, a, alice, "2026-10-02", "10:30")
	if err := a.BlockSlot(ctx, owner, SlotReq{ArtistID: "a2", Date: "2026-10-02", Time: "09:00"}); err != nil {
		t.Fatal(err)
	}
	err = a.BlockSlot(ctx, owner, SlotReq{ArtistID: "a1", Date: "2026-10-02", Time: "09:00"})
	wantCode(t, err, "INVALID_STATE")

	ds, err := a.DaySchedule(ctx, owner, "2026-10-02")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Cells) != len(slotTimes)*len(artists) {
		t.Fatalf("cells = %d", len(ds.Cells))
	}
	cell := func(artist, tm string) ScheduleCell {
		for _, c := range ds.Cells {
			if c.ArtistID == artist && c.Time == tm {
				return c
			}
		}
		t.Fatalf("no cell %s %s", artist, tm)
		return ScheduleCell{}
	}
	if c := cell("a1", "09:00"); c.State != "booked" || c.Booking.ID != unpaid.ID {
		t.Fatalf("unpaid cell %+v", c)
	}
	p := cell("a1", "10:30")
	if p.State != "pending" || !p.Booking.CanConfirm || p.Booking.Alert != "有过敏" || p.Booking.Note != "过敏：对酒精过敏" || p.Booking.Profile == nil {
		t.Fatalf("pending cell %+v", p.Booking)
	}
	if c := cell("a2", "09:00"); c.State != "blocked" {
		t.Fatalf("blocked cell %+v", c)
	}
	// 待付定金不计入 total
	if ds.Stats != (ScheduleStats{Total: 1, Pending: 1, Free: 24 - 3, Stale: 0}) {
		t.Fatalf("stats %+v", ds.Stats)
	}

	// 到点后：格子变 past，不能再确认
	c.t = time.Date(2026, 10, 2, 10, 30, 0, 0, shanghai)
	_, err = a.ConfirmBooking(ctx, owner, pending.ID)
	wantCode(t, err, "INVALID_STATE")
	err = a.UnblockSlot(ctx, owner, SlotReq{ArtistID: "a2", Date: "2026-10-02", Time: "09:00"})
	wantCode(t, err, "INVALID_STATE")
}

func TestSkinProfile(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	u := user(t, a, "o-alice")
	if me, _ := a.Me(ctx, u); me.SkinProfile != "" || me.Role != "customer" {
		t.Fatalf("fresh user %+v", me)
	}
	_, err := a.UpdateSkinProfile(ctx, u, SkinProfile{Note: strings.Repeat("字", 201)})
	wantCode(t, err, "UNKNOWN")
	if _, err := a.UpdateSkinProfile(ctx, u, SkinProfile{SkinType: "sensitive", Tone: "cool_fair"}); err != nil {
		t.Fatal(err)
	}
	u = user(t, a, "o-alice")
	if me, _ := a.Me(ctx, u); me.SkinProfile != "敏感肌 · 冷白皮" {
		t.Fatalf("summary %q", me.SkinProfile)
	}
	if owner, _ := a.Me(ctx, user(t, a, ownerOpenID)); owner.Role != "owner" {
		t.Fatal("owner role")
	}
}

// ---------- HTTP 层 ----------

func do(t *testing.T, h http.Handler, method, path, openid, body string) (int, map[string]any, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if openid != "" {
		req.Header.Set("X-WX-OPENID", openid)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m, rec.Body.String()
}

func TestHTTP(t *testing.T) {
	a, _ := newTestApp(t)
	h := a.Routes()

	if code, m, _ := do(t, h, "GET", "/v1/me", "", ""); code != 401 || m["code"] != "UNAUTHORIZED" {
		t.Fatalf("no openid: %d %v", code, m)
	}
	if code, m, _ := do(t, h, "POST", "/v1/auth/wx-login", "o-alice", "{}"); code != 200 || m["me"] == nil {
		t.Fatalf("login: %d %v", code, m)
	}
	if code, _, body := do(t, h, "GET", "/v1/me/skin-profile", "o-alice", ""); code != 200 || strings.TrimSpace(body) != "{}" {
		t.Fatalf("empty profile should be {}: %s", body)
	}
	code, m, _ := do(t, h, "POST", "/v1/bookings", "o-alice", `{"serviceId":"s1","artistId":"a1","date":"2026-10-03","time":"10:30"}`)
	if code != 200 {
		t.Fatalf("create: %d %v", code, m)
	}
	id := m["booking"].(map[string]any)["id"].(string)
	if code, m, _ := do(t, h, "POST", "/v1/bookings", "o-bob", `{"serviceId":"s1","artistId":"a1","date":"2026-10-03","time":"10:30"}`); code != 409 || m["code"] != "SLOT_TAKEN" {
		t.Fatalf("conflict: %d %v", code, m)
	}
	// mine 不能被当成 {id}
	if code, _, body := do(t, h, "GET", "/v1/bookings/mine?scope=upcoming", "o-alice", ""); code != 200 || !strings.Contains(body, id) {
		t.Fatalf("mine: %d %s", code, body)
	}
	if code, m, _ := do(t, h, "POST", "/v1/bookings/"+id+"/fake-paid", "o-alice", ""); code != 200 || m["status"] != StatusPendingConfirm {
		t.Fatalf("fake-paid: %d %v", code, m)
	}
	if code, _, body := do(t, h, "GET", "/v1/bookings/mine?scope=past", "o-alice", ""); code != 200 || strings.TrimSpace(body) != "[]" {
		t.Fatalf("past should be []: %s", body)
	}
	if code, m, _ := do(t, h, "GET", "/v1/owner/dates", "o-alice", ""); code != 403 || m["code"] != "FORBIDDEN" {
		t.Fatalf("owner: %d %v", code, m)
	}
	if code, _, _ := do(t, h, "DELETE", "/v1/owner/blocks", ownerOpenID, `{"artistId":"a1","date":"2026-10-03","time":"09:00"}`); code != 200 {
		t.Fatalf("unblock: %d", code)
	}
	if code, m, _ := do(t, h, "GET", "/v1/nope", "o-alice", ""); code != 404 || m["code"] != "NOT_FOUND" {
		t.Fatalf("404: %d %v", code, m)
	}
}
