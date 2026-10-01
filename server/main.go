// 鲸屿美妆后端。部署在微信云托管，小程序用 wx.cloud.callContainer 调用，接口契约见 src/api/contract.ts。
//
// 环境变量：
//
//	PORT            监听端口，默认 80（云托管默认端口）
//	MYSQL_ADDRESS   云托管 MySQL 内网地址，例如 10.0.0.1:3306；开通 MySQL 后控制台会给服务注入
//	MYSQL_USERNAME  / MYSQL_PASSWORD 同上
//	MYSQL_DATABASE  库名，默认 jingyu，不存在会自动创建
//	STORE=memory    不连数据库，数据放内存（本地开发用，重启就没了）
//	OWNER_OPENIDS   店主的 openid，多个用逗号分隔
//	PAY_MODE        必须显式设置。fake：还没有商户号时联调用，不收钱；wxpay：微信支付（还没实现）
//	FAKE_PAY_OPENIDS  PAY_MODE=fake 时，除店主外还能用假支付的测试人员 openid，逗号分隔
//	SUBSCRIBE_MSG   订阅消息怎么发。wx：走云托管开放接口服务（连 MySQL 时默认）；log：只打日志（STORE=memory 时默认）
//	                用 wx 要在云托管控制台“云调用 → 微信令牌权限配置”里加上 /cgi-bin/message/subscribe/send
//	MINIPROGRAM_STATE  客人点消息打开哪个版本：formal（默认）/ trial（体验版）/ developer（开发版）
//	CRON_TOKEN      设了才开放 POST /cron/reminders，请求头 X-Cron-Token 要等于它。
//	                服务最小副本数为 0 时进程会被停掉，内置的每分钟定时器不跑，可以用外部定时器调这个接口补发提醒
//
// 假支付会让预约不付钱就变成“已付定金”，正式开放给客人之前必须换成 wxpay。
// 为了防止忘记，PAY_MODE 没有默认值；fake 模式下也只有店主和 FAKE_PAY_OPENIDS 里的人能用。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	var store Store
	if env("STORE", "") == "memory" {
		log.Print("STORE=memory：数据只在内存里，重启就没了")
		store = newMemStore()
	} else {
		addr := os.Getenv("MYSQL_ADDRESS")
		if addr == "" {
			log.Fatal("没有 MYSQL_ADDRESS。本地开发请设 STORE=memory")
		}
		s, err := openMySQL(addr, os.Getenv("MYSQL_USERNAME"), os.Getenv("MYSQL_PASSWORD"), env("MYSQL_DATABASE", "jingyu"))
		if err != nil {
			log.Fatalf("连接 MySQL 失败: %v", err)
		}
		store = s
	}

	owners := openIDSet(os.Getenv("OWNER_OPENIDS"))
	payMode := os.Getenv("PAY_MODE")
	switch payMode {
	case "fake":
		log.Print("PAY_MODE=fake：假支付，不收钱。正式开放给客人前必须换成微信支付")
	case "":
		log.Fatal("没有设置 PAY_MODE。还没有商户号时设 PAY_MODE=fake 联调；正式上线要接微信支付")
	default:
		log.Fatalf("PAY_MODE=%s 还没实现，现在只支持 fake", payMode)
	}
	// 能用假支付的人：店主 + 测试人员
	fakePayers := openIDSet(os.Getenv("FAKE_PAY_OPENIDS"))
	for id := range owners {
		fakePayers[id] = true
	}

	msgMode := env("SUBSCRIBE_MSG", "wx")
	if env("STORE", "") == "memory" {
		msgMode = env("SUBSCRIBE_MSG", "log")
	}
	var notifier Notifier
	switch msgMode {
	case "wx":
		notifier = &wxNotifier{
			base:   env("WX_API_BASE", "http://api.weixin.qq.com"),
			state:  env("MINIPROGRAM_STATE", "formal"),
			client: &http.Client{Timeout: 10 * time.Second},
		}
	case "log":
		log.Print("SUBSCRIBE_MSG=log：订阅消息只打日志，不发给客人")
		notifier = logNotifier{}
	default:
		log.Fatalf("SUBSCRIBE_MSG=%s 不认识，只支持 wx / log", msgMode)
	}

	app := &App{
		store: store, now: time.Now, owners: owners, fakePay: true, fakePayers: fakePayers,
		notifier: notifier, cronToken: os.Getenv("CRON_TOKEN"),
	}

	// 定时任务：实例活着的时候每分钟流转一次预约（每个请求前也会跑），顺便发到店提醒
	go func() {
		for range time.Tick(time.Minute) {
			ctx := context.Background()
			if err := app.Sweep(ctx); err != nil {
				log.Printf("sweep: %v", err)
			}
			if err := app.SendReminders(ctx); err != nil {
				log.Printf("reminders: %v", err)
			}
		}
	}()

	addr := ":" + env("PORT", "80")
	log.Printf("listening on %s, owners=%d, pay=%s, fakePayers=%d, msg=%s", addr, len(owners), payMode, len(fakePayers), msgMode)
	srv := &http.Server{Addr: addr, Handler: logRequests(app.Routes()), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

// openIDSet 解析逗号分隔的 openid
func openIDSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out[id] = true
		}
	}
	return out
}

// logRequests 打一行访问日志，带上 openid，方便在云托管日志里找到店主的 openid
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/healthz" {
			log.Printf("%s %s openid=%s %s", r.Method, r.URL.RequestURI(), r.Header.Get("X-WX-OPENID"), time.Since(start).Round(time.Millisecond))
		}
	})
}
