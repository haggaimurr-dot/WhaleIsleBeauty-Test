package main

import (
	"context"
	"testing"
)

// ownerNotifyApp 模板 ID 还没配时不发，测试里临时填一个
func ownerNotifyApp(t *testing.T) (*App, *clock, *fakeNotifier) {
	old := tmplOwner
	tmplOwner = "tmpl-owner"
	t.Cleanup(func() { tmplOwner = old })
	return notifyApp(t)
}

func addQuota(t *testing.T, a *App, n int) int {
	t.Helper()
	st, err := a.AddOwnerNotify(context.Background(), user(t, a, ownerOpenID), OwnerNotifyReq{Count: n})
	if err != nil {
		t.Fatal(err)
	}
	return st.Quota
}

func quota(t *testing.T, a *App) int {
	t.Helper()
	st, err := a.OwnerNotify(context.Background(), user(t, a, ownerOpenID))
	if err != nil {
		t.Fatal(err)
	}
	return st.Quota
}

// ownerMsgs 只取发给店主的
func ownerMsgs(n *fakeNotifier) []SubscribeMsg {
	var out []SubscribeMsg
	for _, m := range n.take() {
		if m.TemplateID == tmplOwner {
			out = append(out, m)
		}
	}
	return out
}

func TestOwnerNotifyQuota(t *testing.T) {
	a, _, _ := ownerNotifyApp(t)
	ctx := context.Background()
	if quota(t, a) != 0 {
		t.Fatal("额度从 0 开始")
	}
	if addQuota(t, a, 1) != 1 || addQuota(t, a, 5) != 6 {
		t.Fatal("额度要累加")
	}
	for _, n := range []int{0, -1, 6} {
		_, err := a.AddOwnerNotify(ctx, user(t, a, ownerOpenID), OwnerNotifyReq{Count: n})
		wantCode(t, err, "UNKNOWN")
	}
	alice := user(t, a, "o-alice")
	_, err := a.OwnerNotify(ctx, alice)
	wantCode(t, err, "FORBIDDEN")
	_, err = a.AddOwnerNotify(ctx, alice, OwnerNotifyReq{Count: 1})
	wantCode(t, err, "FORBIDDEN")
}

func TestOwnerNotifiedOnPaidRescheduleCancel(t *testing.T) {
	a, _, n := ownerNotifyApp(t)
	ctx := context.Background()
	addQuota(t, a, 5)
	alice := user(t, a, "o-alice")
	b := paid(t, a, alice, "2026-10-03", "13:30")
	// 支付回调重复到达不重复发
	if _, err := a.MarkPaid(ctx, alice, b.ID); err != nil {
		t.Fatal(err)
	}
	got := ownerMsgs(n)
	if len(got) != 1 {
		t.Fatalf("want 1 owner message, got %d", len(got))
	}
	m := got[0]
	if m.ToUser != ownerOpenID || m.Page != ownerMsgPage+"?date=2026-10-03" {
		t.Fatalf("unexpected %+v", m)
	}
	if val(m, ownerKeyService) != "韩式上镜妆（含发型）" || val(m, ownerKeyTime) != "2026年10月03日 13:30" ||
		val(m, ownerKeyStatus) != "新预约" || val(m, ownerKeyCustomer) != alice.Nickname+" · 小鲸" {
		t.Fatalf("unexpected data %v", m.Data)
	}

	if _, err := a.RescheduleBooking(ctx, alice, b.ID, SlotReq{ArtistID: "a1", Date: "2026-10-04", Time: "13:30"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CancelBooking(ctx, alice, b.ID); err != nil {
		t.Fatal(err)
	}
	got = ownerMsgs(n)
	if len(got) != 2 || val(got[0], ownerKeyStatus) != "已改期" || val(got[0], ownerKeyTime) != "2026年10月04日 13:30" ||
		val(got[1], ownerKeyStatus) != "已取消" {
		t.Fatalf("unexpected %v", got)
	}
	if quota(t, a) != 2 {
		t.Fatalf("发了 3 条，额度应剩 2，实际 %d", quota(t, a))
	}
}

func TestOwnerNotNotifiedForUnpaid(t *testing.T) {
	a, _, n := ownerNotifyApp(t)
	ctx := context.Background()
	addQuota(t, a, 5)
	alice := user(t, a, "o-alice")
	b := book(t, a, alice, "2026-10-03", "13:30")
	if _, err := a.RescheduleBooking(ctx, alice, b.ID, SlotReq{ArtistID: "a1", Date: "2026-10-04", Time: "13:30"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CancelBooking(ctx, alice, b.ID); err != nil {
		t.Fatal(err)
	}
	if got := ownerMsgs(n); len(got) != 0 {
		t.Fatalf("没付定金的不打扰店主，发了 %v", got)
	}
	if quota(t, a) != 5 {
		t.Fatal("额度不该用掉")
	}
}

func TestOwnerNotifyQuotaRunsOut(t *testing.T) {
	a, _, n := ownerNotifyApp(t)
	addQuota(t, a, 1)
	alice := user(t, a, "o-alice")
	paid(t, a, alice, "2026-10-03", "13:30")
	paid(t, a, alice, "2026-10-03", "16:30")
	if got := ownerMsgs(n); len(got) != 1 {
		t.Fatalf("额度只有 1，发了 %d", len(got))
	}
	if quota(t, a) != 0 {
		t.Fatal("额度用完是 0")
	}
}

func TestOwnerNotifyErrors(t *testing.T) {
	a, _, n := ownerNotifyApp(t)
	addQuota(t, a, 3)
	alice := user(t, a, "o-alice")
	// 网络错误：没发出去，额度退回
	n.fail = []error{context.DeadlineExceeded}
	paid(t, a, alice, "2026-10-03", "13:30")
	if quota(t, a) != 3 {
		t.Fatalf("没发出去要退回额度，实际 %d", quota(t, a))
	}
	// 微信说没订阅：后端记的额度不准了，清零让店主重新续
	n.fail = []error{&WxError{Code: 43101, Msg: "user refuse to accept the msg"}}
	paid(t, a, alice, "2026-10-03", "16:30")
	if quota(t, a) != 0 {
		t.Fatalf("43101 要清零，实际 %d", quota(t, a))
	}
}

func TestOwnerNotifySkippedWithoutTemplate(t *testing.T) {
	a, _, n := notifyApp(t) // tmplOwner 为空
	addQuota(t, a, 2)
	paid(t, a, user(t, a, "o-alice"), "2026-10-03", "13:30")
	if got := n.take(); len(got) != 0 {
		t.Fatalf("模板没配不发，发了 %v", got)
	}
	if quota(t, a) != 2 {
		t.Fatal("没发就不扣额度")
	}
}
