-- 店主的预约变动提醒额度：订阅消息是一次性的，店主每同意一次加一，每发一条减一，微信说没订阅时清零。
-- 店主由环境变量 OWNER_OPENIDS 决定，所以按 openid 记。
CREATE TABLE owner_notify (
  openid     VARCHAR(64) NOT NULL PRIMARY KEY,
  quota      INT         NOT NULL DEFAULT 0,
  updated_at DATETIME    NOT NULL
) DEFAULT CHARSET=utf8mb4;
