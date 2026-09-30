package main

import "embed"

// 建表脚本打包进二进制，启动时按文件名顺序执行还没跑过的（记录在 schema_migrations 表里）
//
//go:embed migrations/*.sql
var migrationFS embed.FS
