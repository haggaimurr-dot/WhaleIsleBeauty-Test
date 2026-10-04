-- 退款进度：发起退款的时间。refunded_at 改记到账时间，两个都有值且到账时间已过才算退回了。
-- 这之前退的只有 refunded_at，当作发起即到账。
ALTER TABLE bookings ADD COLUMN refund_requested_at DATETIME NULL AFTER paid_at;
