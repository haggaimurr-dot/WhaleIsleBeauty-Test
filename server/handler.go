package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

// 路由和请求解析。路径对应 src/api/contract.ts 里的注释，全部挂在 /v1 下。

type ctxKey struct{}

func userOf(r *http.Request) *User { return r.Context().Value(ctxKey{}).(*User) }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var ae *ApiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, map[string]string{"code": ae.Code, "message": ae.Message})
		return
	}
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "UNKNOWN", "message": ""})
}

func decode(r *http.Request, v any) error {
	if r.ContentLength == 0 {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return badRequest("请求格式不对")
	}
	return nil
}

// reply 把 (结果, 错误) 写成响应
func reply[T any](w http.ResponseWriter, r *http.Request, v T, err error) {
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// withUser 云托管网关会在请求头里带 X-WX-OPENID，按它找到或创建客人；顺便跑一次到期预约的流转
func (a *App) withUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		openid := strings.TrimSpace(r.Header.Get("X-WX-OPENID"))
		if openid == "" {
			writeErr(w, r, apiErr("UNAUTHORIZED", "登录已过期，请重新进入小程序"))
			return
		}
		if err := a.Sweep(r.Context()); err != nil {
			log.Printf("sweep: %v", err)
		}
		u, err := a.UserFor(r.Context(), openid)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	h := func(pattern string, f http.HandlerFunc) { mux.HandleFunc(pattern, a.withUser(f)) }

	// 云托管的健康检查
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	// 外部定时器补发到店提醒（服务缩到 0 个实例时内置定时器不跑）。不经过小程序，靠口令保护
	if a.cronToken != "" {
		mux.HandleFunc("POST /cron/reminders", func(w http.ResponseWriter, r *http.Request) {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Cron-Token")), []byte(a.cronToken)) != 1 {
				writeErr(w, r, errForbidden)
				return
			}
			if err := a.Sweep(r.Context()); err != nil {
				writeErr(w, r, err)
				return
			}
			reply(w, r, struct{}{}, a.SendReminders(r.Context()))
		})
	}

	// ---------- 登录和我的 ----------
	me := func(w http.ResponseWriter, r *http.Request) {
		m, err := a.Me(r.Context(), userOf(r))
		reply(w, r, m, err)
	}
	h("POST /v1/auth/wx-login", func(w http.ResponseWriter, r *http.Request) {
		m, err := a.Me(r.Context(), userOf(r))
		reply(w, r, map[string]Me{"me": m}, err)
	})
	h("GET /v1/me", me)
	h("GET /v1/me/skin-profile", func(w http.ResponseWriter, r *http.Request) {
		reply(w, r, userOf(r).Skin, nil)
	})
	h("PUT /v1/me/skin-profile", func(w http.ResponseWriter, r *http.Request) {
		var req SkinProfile
		if err := decode(r, &req); err != nil {
			writeErr(w, r, err)
			return
		}
		p, err := a.UpdateSkinProfile(r.Context(), userOf(r), req)
		reply(w, r, p, err)
	})

	// ---------- 基础资料 ----------
	h("GET /v1/artists", func(w http.ResponseWriter, r *http.Request) { reply(w, r, artists, nil) })
	h("GET /v1/services", func(w http.ResponseWriter, r *http.Request) { reply(w, r, services, nil) })
	h("GET /v1/services/{id}", func(w http.ResponseWriter, r *http.Request) {
		if s := findService(r.PathValue("id")); s != nil {
			reply(w, r, s, nil)
		} else {
			writeErr(w, r, errNotFound)
		}
	})
	h("GET /v1/works", func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("category")
		out := []Work{}
		for _, wk := range works {
			if c == "" || wk.Category == c {
				out = append(out, wk)
			}
		}
		reply(w, r, out, nil)
	})
	h("GET /v1/dates", func(w http.ResponseWriter, r *http.Request) { reply(w, r, a.datesFrom(1, bookableDays), nil) })
	h("GET /v1/shop", func(w http.ResponseWriter, r *http.Request) { reply(w, r, shop, nil) })

	// ---------- 客人端预约 ----------
	h("GET /v1/slots", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		v, err := a.ListSlots(r.Context(), userOf(r), q.Get("artistId"), q.Get("date"), q.Get("excludeBookingId"))
		reply(w, r, v, err)
	})
	h("POST /v1/bookings", func(w http.ResponseWriter, r *http.Request) {
		var req CreateBookingReq
		if err := decode(r, &req); err != nil {
			writeErr(w, r, err)
			return
		}
		v, err := a.CreateBooking(r.Context(), userOf(r), req)
		reply(w, r, v, err)
	})
	h("GET /v1/bookings/mine", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.ListMyBookings(r.Context(), userOf(r), r.URL.Query().Get("scope"))
		reply(w, r, v, err)
	})
	h("GET /v1/bookings/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.GetBooking(r.Context(), userOf(r), r.PathValue("id"))
		reply(w, r, v, err)
	})
	h("POST /v1/bookings/{id}/pay", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.ResumePayment(r.Context(), userOf(r), r.PathValue("id"))
		reply(w, r, v, err)
	})
	h("POST /v1/bookings/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.CancelBooking(r.Context(), userOf(r), r.PathValue("id"))
		reply(w, r, v, err)
	})
	h("POST /v1/bookings/{id}/reschedule", func(w http.ResponseWriter, r *http.Request) {
		var req SlotReq
		if err := decode(r, &req); err != nil {
			writeErr(w, r, err)
			return
		}
		v, err := a.RescheduleBooking(r.Context(), userOf(r), r.PathValue("id"), req)
		reply(w, r, v, err)
	})
	if a.fakePay {
		// 联调用：代替微信支付回调。不在契约里，接真支付后去掉
		h("POST /v1/bookings/{id}/fake-paid", func(w http.ResponseWriter, r *http.Request) {
			v, err := a.FakePaid(r.Context(), userOf(r), r.PathValue("id"))
			reply(w, r, v, err)
		})
	}

	// ---------- 店主端 ----------
	h("GET /v1/owner/dates", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.ScheduleDates(userOf(r))
		reply(w, r, v, err)
	})
	h("GET /v1/owner/schedule", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.DaySchedule(r.Context(), userOf(r), r.URL.Query().Get("date"))
		reply(w, r, v, err)
	})
	h("POST /v1/owner/bookings/{id}/confirm", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.ConfirmBooking(r.Context(), userOf(r), r.PathValue("id"))
		reply(w, r, v, err)
	})
	slot := func(f func(context.Context, *User, SlotReq) error) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var req SlotReq
			if err := decode(r, &req); err != nil {
				writeErr(w, r, err)
				return
			}
			reply(w, r, struct{}{}, f(r.Context(), userOf(r), req))
		}
	}
	h("PUT /v1/owner/blocks", slot(a.BlockSlot))
	h("DELETE /v1/owner/blocks", slot(a.UnblockSlot))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, r, errNotFound) })
	return mux
}
