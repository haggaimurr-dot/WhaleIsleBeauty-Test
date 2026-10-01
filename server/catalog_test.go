package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// 店主维护资料，用例和前端 mock.catalog.test.ts 对齐

var (
	testArtist  = Artist{Name: "  小满 ", Years: 3, Avatar: "placeholder:g2"}
	testService = Service{
		Name: "毕业照妆", Summary: "适合毕业照", Category: "camera", DurationMin: 75, Price: 19800, Deposit: 3000,
		Includes: []string{"底妆", " ", "眼妆"}, Tags: []string{"上镜"}, Cover: "whatever", Images: []string{"placeholder:g6"},
	}
	testWork = Work{Title: "夏日毕业照", Category: "camera", ArtistID: "a1", Image: "placeholder:g5", Ratio: 1.3, DurationText: "约 75 分钟"}
)

func wantMsg(t *testing.T, err error, msg string) {
	t.Helper()
	ae, ok := err.(*ApiError)
	if !ok || ae.Message != msg {
		t.Fatalf("want %q, got %v", msg, err)
	}
}

// must 出错时 panic，测试会失败并带上调用栈（Go 不能把多返回值和 t 一起传）
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func ownerService(t *testing.T, a *App, owner *User, id string) Service {
	t.Helper()
	for _, s := range must(a.OwnerServices(context.Background(), owner)) {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("no service %s", id)
	return Service{}
}

func TestCatalogOwnerOnly(t *testing.T) {
	a, _ := newTestApp(t)
	alice := user(t, a, "o-alice")
	_, err := a.UpdateShop(context.Background(), alice, seedShop)
	wantCode(t, err, "FORBIDDEN")
	_, err = a.CreateArtist(context.Background(), alice, testArtist)
	wantCode(t, err, "FORBIDDEN")
	err = a.DeleteWork(context.Background(), alice, "w1")
	wantCode(t, err, "FORBIDDEN")
}

func TestUpdateShop(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner := context.Background(), user(t, a, ownerOpenID)
	s := seedShop
	s.Phone, s.OpenHours = " 138 0000 0000 ", "10:00–20:00"
	saved := must(a.UpdateShop(ctx, owner, s))
	if saved.Phone != "138 0000 0000" || must(a.Shop(ctx)).OpenHours != "10:00–20:00" {
		t.Fatalf("shop %+v", saved)
	}

	bad := seedShop
	bad.Name = " "
	_, err := a.UpdateShop(ctx, owner, bad)
	wantMsg(t, err, "店名还没填")
	bad = seedShop
	bad.Phone = "打电话"
	_, err = a.UpdateShop(ctx, owner, bad)
	wantCode(t, err, "UNKNOWN")
	bad = seedShop
	bad.Latitude, bad.Longitude = 0, 0
	_, err = a.UpdateShop(ctx, owner, bad)
	wantMsg(t, err, "地图位置还没选")
}

func TestArtists(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner, alice := context.Background(), user(t, a, ownerOpenID), user(t, a, testerOpenID)

	ar := must(a.CreateArtist(ctx, owner, testArtist))
	list := must(a.Artists(ctx))
	if ar.Name != "小满" || list[len(list)-1].ID != ar.ID {
		t.Fatalf("new artist %+v, list %+v", ar, list)
	}
	ds := must(a.DaySchedule(ctx, owner, "2026-10-02"))
	if len(ds.Artists) != 4 || len(ds.Cells) != len(slotTimes)*4 {
		t.Fatalf("schedule artists %+v", ds.Artists)
	}

	// 改名不影响已经下的单
	b := book(t, a, alice, "2026-10-03", "09:00")
	must(a.UpdateArtist(ctx, owner, "a1", Artist{Name: "大鲸", Years: 9, Avatar: "placeholder:g1"}))
	if got := must(a.GetBooking(ctx, alice, b.ID)); got.ArtistName != "小鲸" {
		t.Fatalf("booking artist %s", got.ArtistName)
	}
	_, err := a.UpdateArtist(ctx, owner, "nope", testArtist)
	wantCode(t, err, "NOT_FOUND")

	// 有没结束的预约、名下有作品时不能删
	resp := must(a.CreateBooking(ctx, alice, CreateBookingReq{ServiceID: "s1", ArtistID: ar.ID, Date: "2026-10-03", Time: "10:30"}))
	wantCode(t, a.DeleteArtist(ctx, owner, ar.ID), "INVALID_STATE")
	must(a.CancelBooking(ctx, alice, resp.Booking.ID))
	w := testWork
	w.ArtistID = ar.ID
	wk := must(a.CreateWork(ctx, owner, w))
	wantMsg(t, a.DeleteArtist(ctx, owner, ar.ID), "TA 名下还有 1 个作品，先改给别人或删掉")
	if err := a.DeleteWork(ctx, owner, wk.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteArtist(ctx, owner, ar.ID); err != nil {
		t.Fatal(err)
	}
	// 删掉后不能再约 TA
	_, err = a.CreateBooking(ctx, alice, CreateBookingReq{ServiceID: "s1", ArtistID: ar.ID, Date: "2026-10-03", Time: "12:00"})
	wantCode(t, err, "NOT_FOUND")
}

func TestKeepOneArtist(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner := context.Background(), user(t, a, ownerOpenID)
	for _, w := range must(a.OwnerWorks(ctx, owner)) {
		if err := a.DeleteWork(ctx, owner, w.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"a2", "a3"} {
		if err := a.DeleteArtist(ctx, owner, id); err != nil {
			t.Fatal(err)
		}
	}
	wantMsg(t, a.DeleteArtist(ctx, owner, "a1"), "至少要留一位化妆师")
}

func TestServices(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner, alice := context.Background(), user(t, a, ownerOpenID), user(t, a, testerOpenID)

	s := must(a.CreateService(ctx, owner, testService))
	if s.BookedCount != 0 || len(s.Includes) != 2 || s.Cover != "placeholder:g6" {
		t.Fatalf("new service %+v", s)
	}
	bad := testService
	bad.Deposit = 20000
	_, err := a.CreateService(ctx, owner, bad)
	wantMsg(t, err, "定金不能比价格高")
	bad = testService
	bad.DurationMin = 70
	_, err = a.CreateService(ctx, owner, bad)
	wantCode(t, err, "UNKNOWN")

	// 改价只影响之后的新预约，已选择人数沿用
	b := book(t, a, alice, "2026-10-03", "09:00")
	s1 := ownerService(t, a, owner, "s1")
	s1.Price, s1.BookedCount = 32800, 0
	if got := must(a.UpdateService(ctx, owner, "s1", s1)); got.BookedCount != 128 {
		t.Fatalf("booked count %d", got.BookedCount)
	}
	if got := must(a.GetBooking(ctx, alice, b.ID)); got.Price != 29800 {
		t.Fatalf("old booking price %d", got.Price)
	}
	if got := book(t, a, alice, "2026-10-03", "10:30"); got.Price != 32800 {
		t.Fatalf("new booking price %d", got.Price)
	}
}

func TestHideService(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner, alice := context.Background(), user(t, a, ownerOpenID), user(t, a, testerOpenID)
	resp := must(a.CreateBooking(ctx, alice, CreateBookingReq{ServiceID: "s2", ArtistID: "a1", Date: "2026-10-03", Time: "09:00"}))
	must(a.FakePaid(ctx, alice, resp.Booking.ID))

	s2 := ownerService(t, a, owner, "s2")
	s2.Hidden = true
	must(a.UpdateService(ctx, owner, "s2", s2))

	for _, s := range must(a.Services(ctx)) {
		if s.ID == "s2" {
			t.Fatal("hidden service listed")
		}
	}
	if !must(a.Service(ctx, "s2")).Hidden {
		t.Fatal("detail should say hidden")
	}
	_, err := a.CreateBooking(ctx, alice, CreateBookingReq{ServiceID: "s2", ArtistID: "a2", Date: "2026-10-03", Time: "09:00"})
	wantCode(t, err, "INVALID_STATE")
	if got := must(a.GetBooking(ctx, alice, resp.Booking.ID)); got.Status != StatusPendingConfirm {
		t.Fatalf("existing booking %s", got.Status)
	}
	// 客人端作品不再带下架项目，店主那边照常
	for _, w := range must(a.Works(ctx, "")) {
		if w.ID == "w2" && w.ServiceID != "" {
			t.Fatal("customer work should drop hidden service")
		}
	}
	for _, w := range must(a.OwnerWorks(ctx, owner)) {
		if w.ID == "w2" && w.ServiceID != "s2" {
			t.Fatal("owner work should keep service")
		}
	}
}

func TestKeepOneServiceOnSale(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner := context.Background(), user(t, a, ownerOpenID)
	list := must(a.OwnerServices(ctx, owner))
	for _, s := range list[1:] {
		s.Hidden = true
		must(a.UpdateService(ctx, owner, s.ID, s))
	}
	last := list[0]
	last.Hidden = true
	_, err := a.UpdateService(ctx, owner, last.ID, last)
	wantMsg(t, err, "至少要留一个在接预约的项目")
	wantCode(t, a.DeleteService(ctx, owner, last.ID), "INVALID_STATE")
}

func TestDeleteService(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner, alice := context.Background(), user(t, a, ownerOpenID), user(t, a, testerOpenID)
	resp := must(a.CreateBooking(ctx, alice, CreateBookingReq{ServiceID: "s2", ArtistID: "a1", Date: "2026-10-03", Time: "09:00"}))
	wantCode(t, a.DeleteService(ctx, owner, "s2"), "INVALID_STATE")
	must(a.CancelBooking(ctx, alice, resp.Booking.ID))
	if err := a.DeleteService(ctx, owner, "s2"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Service(ctx, "s2"); err == nil {
		t.Fatal("service still there")
	}
	for _, w := range must(a.OwnerWorks(ctx, owner)) {
		if w.ServiceID == "s2" {
			t.Fatalf("work %s still points to s2", w.ID)
		}
	}
	// 取消掉的那条预约还在，项目名是下单时的
	if got := must(a.GetBooking(ctx, alice, resp.Booking.ID)); got.ServiceName != "日常约会妆" {
		t.Fatalf("booking %+v", got)
	}
}

func TestWorks(t *testing.T) {
	a, _ := newTestApp(t)
	ctx, owner := context.Background(), user(t, a, ownerOpenID)
	w := testWork
	w.ServiceID = "s1"
	created := must(a.CreateWork(ctx, owner, w))
	if got := must(a.Works(ctx, "")); got[0].ID != created.ID {
		t.Fatalf("newest first: %s", got[0].ID)
	}
	if got := must(a.Works(ctx, "camera")); got[0].ID != created.ID {
		t.Fatal("category filter")
	}
	// 种子作品的顺序不变
	if got := must(a.Works(ctx, "")); got[1].ID != "w1" || got[len(got)-1].ID != "w8" {
		t.Fatalf("seed order %s … %s", got[1].ID, got[len(got)-1].ID)
	}

	bad := testWork
	bad.ArtistID = "nope"
	_, err := a.CreateWork(ctx, owner, bad)
	wantCode(t, err, "UNKNOWN")
	bad = testWork
	bad.ServiceID = "nope"
	_, err = a.CreateWork(ctx, owner, bad)
	wantCode(t, err, "UNKNOWN")

	upd := testWork
	upd.Title = "改个名"
	got := must(a.UpdateWork(ctx, owner, "w1", upd))
	if got.ID != "w1" || got.Title != "改个名" || got.ServiceID != "" {
		t.Fatalf("updated %+v", got)
	}
	if err := a.DeleteWork(ctx, owner, "w1"); err != nil {
		t.Fatal(err)
	}
	wantCode(t, a.DeleteWork(ctx, owner, "w1"), "NOT_FOUND")
}

func TestCatalogHTTP(t *testing.T) {
	a, _ := newTestApp(t)
	h := a.Routes()

	code, m, body := do(t, h, "POST", "/v1/owner/artists", ownerOpenID, `{"name":"小满","years":3,"avatar":"placeholder:g2"}`)
	if code != http.StatusOK {
		t.Fatalf("create artist %d %s", code, body)
	}
	id, _ := m["id"].(string)
	if code, m, _ := do(t, h, "PUT", "/v1/owner/artists/"+id, ownerOpenID, `{"name":"","years":3,"avatar":"placeholder:g2"}`); code != http.StatusBadRequest || m["message"] != "名字还没填" {
		t.Fatalf("invalid artist %d %v", code, m)
	}
	if code, _, body := do(t, h, "DELETE", "/v1/owner/artists/"+id, ownerOpenID, ""); code != http.StatusOK {
		t.Fatalf("delete artist %d %s", code, body)
	}
	if code, _, _ := do(t, h, "GET", "/v1/owner/services", "o-alice", ""); code != http.StatusForbidden {
		t.Fatalf("customer owner services %d", code)
	}
	if _, m, body := do(t, h, "GET", "/v1/services/s1", "o-alice", ""); m["name"] != "韩式上镜妆（含发型）" {
		t.Fatalf("service detail %s", body)
	}
	shop := `{"name":"鲸屿","address":"蓝山CBD 3329","phone":"020-1","openHours":"10:00–20:00","latitude":23.1,"longitude":113.2}`
	if code, _, body := do(t, h, "PUT", "/v1/owner/shop", ownerOpenID, shop); code != http.StatusOK {
		t.Fatalf("shop %d %s", code, body)
	}
	if _, m, body := do(t, h, "GET", "/v1/shop", "o-alice", ""); m["name"] != "鲸屿" {
		t.Fatalf("shop after update %s", body)
	}
	// 数组字段为空时是 [] 不是 null
	svc := `{"name":"试妆","summary":"先试一次","category":"bridal","durationMin":60,"price":10000,"deposit":2000,"images":["placeholder:g1"]}`
	if code, _, body := do(t, h, "POST", "/v1/owner/services", ownerOpenID, svc); code != http.StatusOK || !strings.Contains(body, `"includes":[]`) {
		t.Fatalf("create service %d %s", code, body)
	}
}
