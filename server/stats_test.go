package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// 往存储里直接放一条以前的预约（不走下单流程），统计历史月份用
func history(t *testing.T, a *App, userID, date, status string, deposit int, paid bool) {
	t.Helper()
	start, _ := parseStart(date, "13:30")
	b := &BookingRow{
		ID: newID("h"), UserID: userID, ServiceID: "s1", ServiceName: "上镜妆", ArtistID: "a1", ArtistName: "小鲸",
		Date: date, Time: "13:30", StartAt: start.UTC(), EndAt: start.Add(90 * time.Minute).UTC(), DurationMin: 90,
		Price: 29800, Deposit: deposit, Status: status, CreatedAt: start.UTC(),
	}
	if paid {
		at := start.Add(-48 * time.Hour).UTC()
		b.PaidAt = &at
	}
	if status == StatusCancelled {
		b.CancelReason = ReasonCustomer
	}
	if err := a.store.InsertBooking(context.Background(), b); err != nil {
		t.Fatal(err)
	}
}

func statsOf(t *testing.T, a *App, month string) MonthStats {
	t.Helper()
	list, err := a.MonthStats(context.Background(), user(t, a, ownerOpenID))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Month == month {
			return s
		}
	}
	t.Fatalf("no stats for %s", month)
	return MonthStats{}
}

func TestStatsMonthsAndOwnerOnly(t *testing.T) {
	a, _ := newTestApp(t) // 2026-10-01
	list, err := a.MonthStats(context.Background(), user(t, a, ownerOpenID))
	if err != nil {
		t.Fatal(err)
	}
	var months []string
	for _, s := range list {
		months = append(months, s.Month)
		if s != (MonthStats{Month: s.Month}) {
			t.Fatalf("empty store should be all zero, got %+v", s)
		}
	}
	if fmt.Sprint(months) != "[2026-10 2026-09 2026-08 2026-07 2026-06 2026-05]" {
		t.Fatalf("months %v", months)
	}
	_, err = a.MonthStats(context.Background(), user(t, a, "o-alice"))
	wantCode(t, err, "FORBIDDEN")
}

func TestStatsOnlyPaidBookings(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	u := user(t, a, "o-alice")

	ok := paid(t, a, u, "2026-10-03", "09:00")
	book(t, a, u, "2026-10-03", "10:30") // 还没付
	unpaidCancel := book(t, a, u, "2026-10-04", "09:00")
	if _, err := a.CancelBooking(ctx, u, unpaidCancel.ID); err != nil {
		t.Fatal(err)
	}
	paidCancel := paid(t, a, u, "2026-10-05", "09:00")
	if _, err := a.CancelBooking(ctx, u, paidCancel.ID); err != nil {
		t.Fatal(err)
	}

	got := statsOf(t, a, "2026-10")
	want := MonthStats{Month: "2026-10", Bookings: 1, Cancelled: 1, Deposit: ok.Deposit, Customers: 1}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStatsByVisitDateAndReturning(t *testing.T) {
	a, _ := newTestApp(t)
	// x：8 月来过，9 月又来 → 9 月回头客
	history(t, a, "x", "2026-08-10", StatusCompleted, 5000, true)
	history(t, a, "x", "2026-09-05", StatusCompleted, 5000, true)
	// y：9 月第一次来，同月又来一次 → 一位客人，算回头客（第二次之前来过）
	history(t, a, "y", "2026-09-03", StatusCompleted, 3000, true)
	history(t, a, "y", "2026-09-20", StatusCompleted, 3000, true)
	// z：9 月只来一次 → 新客
	history(t, a, "z", "2026-09-12", StatusCompleted, 30000, true)
	// w：4 月来过（不在 6 个月里），6 月再来 → 6 月回头客
	history(t, a, "w", "2026-04-01", StatusCompleted, 5000, true)
	history(t, a, "w", "2026-06-15", StatusCompleted, 5000, true)
	// 9 月：付过定金后取消 1 个，没付就取消 1 个（不算）
	history(t, a, "v", "2026-09-25", StatusCancelled, 5000, true)
	history(t, a, "v", "2026-09-26", StatusCancelled, 5000, false)

	if got, want := statsOf(t, a, "2026-09"), (MonthStats{Month: "2026-09", Bookings: 4, Cancelled: 1, Deposit: 41000, Customers: 3, Returning: 2}); got != want {
		t.Fatalf("09: got %+v, want %+v", got, want)
	}
	if got, want := statsOf(t, a, "2026-08"), (MonthStats{Month: "2026-08", Bookings: 1, Deposit: 5000, Customers: 1}); got != want {
		t.Fatalf("08: got %+v, want %+v", got, want)
	}
	if got, want := statsOf(t, a, "2026-06"), (MonthStats{Month: "2026-06", Bookings: 1, Deposit: 5000, Customers: 1, Returning: 1}); got != want {
		t.Fatalf("06: got %+v, want %+v", got, want)
	}
}

func TestStatsHTTP(t *testing.T) {
	a, _ := newTestApp(t)
	h := a.Routes()
	if code, _, body := do(t, h, "GET", "/v1/owner/stats", ownerOpenID, ""); code != 200 || body[0] != '[' {
		t.Fatalf("owner: %d %s", code, body)
	}
	if code, m, _ := do(t, h, "GET", "/v1/owner/stats", "o-alice", ""); code != 403 || m["code"] != "FORBIDDEN" {
		t.Fatalf("customer: %d %v", code, m)
	}
}
