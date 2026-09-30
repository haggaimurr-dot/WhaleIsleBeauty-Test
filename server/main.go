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
//	PAY_MODE        fake（默认，还没有商户号）或 wxpay
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

	owners := map[string]bool{}
	for _, id := range strings.Split(os.Getenv("OWNER_OPENIDS"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			owners[id] = true
		}
	}
	payMode := env("PAY_MODE", "fake")
	if payMode != "fake" {
		log.Fatalf("PAY_MODE=%s 还没实现，现在只支持 fake", payMode)
	}

	app := &App{store: store, now: time.Now, owners: owners, fakePay: true}

	// 实例活着的时候每分钟也流转一次；每个请求前还会再跑一次
	go func() {
		for range time.Tick(time.Minute) {
			if err := app.Sweep(context.Background()); err != nil {
				log.Printf("sweep: %v", err)
			}
		}
	}()

	addr := ":" + env("PORT", "80")
	log.Printf("listening on %s, owners=%d, pay=%s", addr, len(owners), payMode)
	srv := &http.Server{Addr: addr, Handler: logRequests(app.Routes()), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
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
