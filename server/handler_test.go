package main

import (
	"strings"
	"testing"
)

// 朋友圈单页模式没有 openid：公开资料能看，其余接口仍然 401
func TestPublicWithoutOpenID(t *testing.T) {
	a, _ := newTestApp(t)
	h := a.Routes()

	for _, p := range []string{"/v1/shop", "/v1/artists", "/v1/services", "/v1/services/s1", "/v1/works?category=daily", "/v1/dates"} {
		if code, _, body := do(t, h, "GET", p, "", ""); code != 200 || strings.Contains(body, "UNAUTHORIZED") {
			t.Fatalf("%s: %d %s", p, code, body)
		}
	}
	if code, m, _ := do(t, h, "GET", "/v1/services/nope", "", ""); code != 404 || m["code"] != "NOT_FOUND" {
		t.Fatalf("missing service: %d %v", code, m)
	}
	for _, p := range []string{"GET /v1/slots?artistId=a1&date=2026-10-03", "GET /v1/bookings/mine", "POST /v1/bookings", "GET /v1/owner/dates", "POST /v1/auth/wx-login"} {
		mp := strings.SplitN(p, " ", 2)
		if code, m, _ := do(t, h, mp[0], mp[1], "", ""); code != 401 || m["code"] != "UNAUTHORIZED" {
			t.Fatalf("%s should need openid: %d %v", p, code, m)
		}
	}
	// 用户还是只在登录接口里建
	if n := len(a.store.(*memStore).users); n != 0 {
		t.Fatalf("anonymous requests created %d users", n)
	}
}
