package main

import (
	"context"
	"testing"
	"time"
)

// 店主替客人取消、改期，标记客人没来。现在是 2026-10-01 10:00

func cellOf(t *testing.T, ds DaySchedule, artist, tm string) ScheduleCell {
	t.Helper()
	for _, c := range ds.Cells {
		if c.ArtistID == artist && c.Time == tm {
			return c
		}
	}
	t.Fatalf("no cell %s %s", artist, tm)
	return ScheduleCell{}
}

func schedule(t *testing.T, a *App, date string) DaySchedule {
	t.Helper()
	ds, err := a.DaySchedule(context.Background(), user(t, a, ownerOpenID), date)
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

func TestOwnerCancel(t *testing.T) {
	a, c, n := ownerNotifyApp(t)
	ctx := context.Background()
	alice, owner := user(t, a, "o-alice"), user(t, a, ownerOpenID)
	b := paid(t, a, alice, "2026-10-02", "09:00")
	confirm(t, a, b.ID)
	addQuota(t, a, 3)
	n.take()

	_, err := a.OwnerCancelBooking(ctx, alice, b.ID)
	wantCode(t, err, "FORBIDDEN")
	// 不到 24 小时，客人自己取消不了，店里可以
	c.t = time.Date(2026, 10, 2, 8, 0, 0, 0, shanghai)
	_, err = a.CancelBooking(ctx, alice, b.ID)
	wantCode(t, err, "CANCEL_TOO_LATE")
	got, err := a.OwnerCancelBooking(ctx, owner, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCancelled || got.CancelReason != ReasonCustomer {
		t.Fatalf("unexpected %+v", got)
	}
	if row, _ := a.store.Booking(ctx, b.ID); row.RefundedAt == nil {
		t.Fatal("owner cancel should refund")
	}
	if msgs := n.take(); len(msgs) != 0 {
		t.Fatalf("店里自己取消不发消息，发了 %v", msgs)
	}
	if cell := cellOf(t, schedule(t, a, "2026-10-02"), "a1", "09:00"); cell.State != "free" {
		t.Fatalf("slot should be free, got %+v", cell)
	}
	_, err = a.OwnerCancelBooking(ctx, owner, b.ID)
	wantCode(t, err, "INVALID_STATE")

	// 开始时间过了不能取消
	later := paid(t, a, alice, "2026-10-03", "10:30")
	c.t = time.Date(2026, 10, 3, 10, 30, 0, 0, shanghai)
	_, err = a.OwnerCancelBooking(ctx, owner, later.ID)
	wantCode(t, err, "INVALID_STATE")
}

func TestOwnerReschedule(t *testing.T) {
	a, c, n := ownerNotifyApp(t)
	ctx := context.Background()
	alice, bob, owner := user(t, a, "o-alice"), user(t, a, "o-bob"), user(t, a, ownerOpenID)
	b := paid(t, a, alice, "2026-10-02", "09:00")
	book(t, a, bob, "2026-10-04", "10:30")
	if err := a.BlockSlot(ctx, owner, SlotReq{ArtistID: "a1", Date: "2026-10-04", Time: "12:00"}); err != nil {
		t.Fatal(err)
	}
	addQuota(t, a, 3)
	n.take()

	_, err := a.OwnerRescheduleBooking(ctx, alice, b.ID, SlotReq{ArtistID: "a2", Date: "2026-10-03", Time: "15:00"})
	wantCode(t, err, "FORBIDDEN")
	for _, to := range []SlotReq{
		{ArtistID: "a1", Date: "2026-10-04", Time: "10:30"}, // 别人约了
		{ArtistID: "a1", Date: "2026-10-04", Time: "12:00"}, // 休息
		{ArtistID: "a1", Date: "2026-10-01", Time: "09:00"}, // 已经过去
	} {
		_, err = a.OwnerRescheduleBooking(ctx, owner, b.ID, to)
		wantCode(t, err, "SLOT_TAKEN")
	}
	_, err = a.OwnerRescheduleBooking(ctx, owner, b.ID, SlotReq{ArtistID: "a1", Date: "2026-10-20", Time: "09:00"})
	wantCode(t, err, "INVALID_STATE")

	// 不到 24 小时也能改，可以改到今天；付过定金的直接算确认，给客人发确认，不给店主发提醒
	c.t = time.Date(2026, 10, 2, 8, 0, 0, 0, shanghai)
	got, err := a.OwnerRescheduleBooking(ctx, owner, b.ID, SlotReq{ArtistID: "a2", Date: "2026-10-02", Time: "15:00"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusConfirmed || got.ArtistName != "安安" || got.Date != "2026-10-02" || got.Time != "15:00" {
		t.Fatalf("unexpected %+v", got)
	}
	msgs := n.take()
	if len(msgs) != 1 || msgs[0].TemplateID == tmplOwner || msgs[0].ToUser != "o-alice" {
		t.Fatalf("want one confirm message to alice, got %+v", msgs)
	}
	row, _ := a.store.Booking(ctx, b.ID)
	if want := time.Date(2026, 10, 2, 16, 30, 0, 0, shanghai); !row.EndAt.Equal(want) {
		t.Fatalf("end_at %v", row.EndAt)
	}
	// 原来的时段放出来了
	if cell := cellOf(t, schedule(t, a, "2026-10-02"), "a1", "09:00"); cell.State != "free" {
		t.Fatalf("old slot %+v", cell)
	}

	// 还没付定金的仍是待付定金
	unpaid := book(t, a, bob, "2026-10-05", "09:00")
	got, err = a.OwnerRescheduleBooking(ctx, owner, unpaid.ID, SlotReq{ArtistID: "a1", Date: "2026-10-05", Time: "10:30"})
	if err != nil || got.Status != StatusPendingPayment {
		t.Fatalf("unpaid: %+v %v", got, err)
	}

	// 开始以后不能再改
	c.t = time.Date(2026, 10, 2, 15, 0, 0, 0, shanghai)
	_, err = a.OwnerRescheduleBooking(ctx, owner, b.ID, SlotReq{ArtistID: "a2", Date: "2026-10-03", Time: "15:00"})
	wantCode(t, err, "INVALID_STATE")
}

func TestMarkNoShow(t *testing.T) {
	a, c := newTestApp(t)
	ctx := context.Background()
	alice, owner := user(t, a, "o-alice"), user(t, a, ownerOpenID)
	b := paid(t, a, alice, "2026-10-02", "10:30")
	early := paid(t, a, alice, "2026-10-02", "09:00")
	confirm(t, a, b.ID)
	confirm(t, a, early.ID)

	ds := schedule(t, a, "2026-10-02")
	if br := cellOf(t, ds, "a1", "10:30").Booking; !br.CanChange || br.CanMarkNoShow {
		t.Fatalf("before start %+v", br)
	}
	_, err := a.MarkNoShow(ctx, owner, b.ID)
	wantCode(t, err, "INVALID_STATE")

	// 开始了还没结束：还是已确认，可以标记
	c.t = time.Date(2026, 10, 2, 10, 40, 0, 0, shanghai)
	_ = a.Sweep(ctx)
	ds = schedule(t, a, "2026-10-02")
	if br := cellOf(t, ds, "a1", "10:30").Booking; br.CanChange || !br.CanMarkNoShow || br.Status != StatusConfirmed {
		t.Fatalf("after start %+v", br)
	}
	// 09:00 那个已经结束、自动完成了，格子照样显示，也能标记
	if cell := cellOf(t, ds, "a1", "09:00"); cell.State != "booked" || cell.Booking.Status != StatusCompleted || !cell.Booking.CanMarkNoShow {
		t.Fatalf("completed cell %+v", cell)
	}
	if ds.Stats.Total != 2 {
		t.Fatalf("stats %+v", ds.Stats)
	}
	_, err = a.MarkNoShow(ctx, alice, b.ID)
	wantCode(t, err, "FORBIDDEN")

	got, err := a.MarkNoShow(ctx, owner, early.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCancelled || got.CancelReason != ReasonNoShow {
		t.Fatalf("unexpected %+v", got)
	}
	if row, _ := a.store.Booking(ctx, early.ID); row.RefundedAt == nil {
		t.Fatal("no-show should refund")
	}
	me, _ := a.Me(ctx, alice)
	if me.VisitCount != 0 {
		t.Fatalf("visitCount %d", me.VisitCount)
	}
	ds = schedule(t, a, "2026-10-02")
	if cell := cellOf(t, ds, "a1", "09:00"); cell.State != "booked" || cell.Booking.Status != StatusCancelled || cell.Booking.CanMarkNoShow {
		t.Fatalf("no-show cell %+v", cell)
	}
	if ds.Stats.Total != 1 {
		t.Fatalf("no-show should not count, stats %+v", ds.Stats)
	}
	_, err = a.MarkNoShow(ctx, owner, early.ID)
	wantCode(t, err, "INVALID_STATE")

	// 第二天不能再标记
	c.t = time.Date(2026, 10, 3, 9, 0, 0, 0, shanghai)
	_ = a.Sweep(ctx)
	_, err = a.MarkNoShow(ctx, owner, b.ID)
	wantCode(t, err, "INVALID_STATE")

	// 统计：没来算在取消里，定金不算收入
	s := statsOf(t, a, "2026-10")
	if s.Bookings != 1 || s.Cancelled != 1 || s.NoShow != 1 || s.Deposit != b.Deposit {
		t.Fatalf("stats %+v", s)
	}
}

func TestOwnerActionsHTTP(t *testing.T) {
	a, _ := newTestApp(t)
	h := a.Routes()
	b := paid(t, a, user(t, a, "o-alice"), "2026-10-03", "10:30")
	if code, body, _ := do(t, h, "POST", "/v1/owner/bookings/"+b.ID+"/reschedule", ownerOpenID, `{"artistId":"a2","date":"2026-10-03","time":"15:00"}`); code != 200 || body["status"] != StatusConfirmed {
		t.Fatalf("reschedule %d %v", code, body)
	}
	if code, body, _ := do(t, h, "POST", "/v1/owner/bookings/"+b.ID+"/no-show", ownerOpenID, ""); code != 409 || body["code"] != "INVALID_STATE" {
		t.Fatalf("no-show %d %v", code, body)
	}
	if code, body, _ := do(t, h, "POST", "/v1/owner/bookings/"+b.ID+"/cancel", ownerOpenID, ""); code != 200 || body["cancelReason"] != ReasonCustomer {
		t.Fatalf("cancel %d %v", code, body)
	}
}
