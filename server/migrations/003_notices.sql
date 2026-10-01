-- 订阅消息发送记录：同一条预约、同一个时段、同一种消息只发一次（notice_key = 预约ID|种类|日期|时间）。
-- result 为空表示正在发；sent 表示发出去了；其他是微信返回的错误，比如客人没订阅（43101）。
CREATE TABLE notices (
  notice_key  VARCHAR(160) NOT NULL PRIMARY KEY,
  booking_id  VARCHAR(32)  NOT NULL,
  kind        VARCHAR(20)  NOT NULL,
  result      VARCHAR(80)  NOT NULL DEFAULT '',
  created_at  DATETIME     NOT NULL,
  finished_at DATETIME NULL,
  KEY idx_booking (booking_id)
) DEFAULT CHARSET=utf8mb4;
