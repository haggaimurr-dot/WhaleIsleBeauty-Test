-- 基础资料：门店、化妆师、项目、作品，每条一行 JSON（字段和 src/api/types.ts 一致）。
-- 表是空的时候服务启动会写入 seed.go 的初始数据，之后由店主在小程序里维护。
-- sort 只在新建时写入：化妆师和项目按它升序，作品按它降序（最新的在前）。
CREATE TABLE catalog (
  kind       VARCHAR(16) NOT NULL,
  id         VARCHAR(32) NOT NULL,
  sort       BIGINT      NOT NULL,
  data       JSON        NOT NULL,
  updated_at DATETIME    NOT NULL,
  PRIMARY KEY (kind, id)
) DEFAULT CHARSET=utf8mb4;
