-- 初始表结构。用 IF NOT EXISTS：改成版本化脚本之前已经建过表的库，执行这个文件不会有变化。
-- 时间一律按 UTC 存 DATETIME。已经上线执行过的文件不要改，改表结构请加新文件。

CREATE TABLE IF NOT EXISTS users (
  id              VARCHAR(32)  NOT NULL PRIMARY KEY,
  openid          VARCHAR(64)  NOT NULL,
  nickname        VARCHAR(64)  NOT NULL,
  avatar          VARCHAR(255) NOT NULL,
  skin_type       VARCHAR(20)  NOT NULL DEFAULT '',
  tone            VARCHAR(20)  NOT NULL DEFAULT '',
  allergies       VARCHAR(1000) NOT NULL DEFAULT '',
  skin_note       VARCHAR(1000) NOT NULL DEFAULT '',
  skin_updated_at DATETIME NULL,
  created_at      DATETIME NOT NULL,
  UNIQUE KEY uk_openid (openid)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS bookings (
  id            VARCHAR(32)  NOT NULL PRIMARY KEY,
  user_id       VARCHAR(32)  NOT NULL,
  service_id    VARCHAR(32)  NOT NULL,
  service_name  VARCHAR(64)  NOT NULL,
  artist_id     VARCHAR(32)  NOT NULL,
  artist_name   VARCHAR(64)  NOT NULL,
  date          CHAR(10)     NOT NULL,
  time          CHAR(5)      NOT NULL,
  start_at      DATETIME     NOT NULL,
  end_at        DATETIME     NOT NULL,
  duration_min  INT          NOT NULL,
  price         INT          NOT NULL,
  deposit       INT          NOT NULL,
  status        VARCHAR(20)  NOT NULL,
  occasion      VARCHAR(20)  NOT NULL DEFAULT '',
  skin_type     VARCHAR(20)  NOT NULL DEFAULT '',
  note          VARCHAR(1000) NOT NULL DEFAULT '',
  created_at    DATETIME     NOT NULL,
  pay_deadline  DATETIME NULL,
  paid_at       DATETIME NULL,
  refunded_at   DATETIME NULL,
  cancel_reason VARCHAR(20)  NOT NULL DEFAULT '',
  active_slot   VARCHAR(64) NULL,
  UNIQUE KEY uk_active_slot (active_slot),
  KEY idx_user (user_id),
  KEY idx_date (date),
  KEY idx_status_start (status, start_at)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS blocks (
  slot_key   VARCHAR(64) NOT NULL PRIMARY KEY,
  artist_id  VARCHAR(32) NOT NULL,
  date       CHAR(10)    NOT NULL,
  time       CHAR(5)     NOT NULL,
  created_at DATETIME    NOT NULL,
  KEY idx_date (date)
) DEFAULT CHARSET=utf8mb4;
