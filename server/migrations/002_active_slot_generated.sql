-- 把 active_slot 改成生成列：由数据库根据 status 算出来，应用代码不再负责写它。
-- 进行中的预约（待付定金、待确认、已确认）才有值，唯一索引保证同一 (化妆师, 日期, 时间) 同时只有一个；
-- 结束的预约是 NULL，唯一索引允许多个 NULL。
ALTER TABLE bookings DROP COLUMN active_slot;
ALTER TABLE bookings
  ADD COLUMN active_slot VARCHAR(64) AS (
    IF(status IN ('pending_payment', 'pending_confirm', 'confirmed'), CONCAT(artist_id, '|', date, '|', time), NULL)
  ) STORED,
  ADD UNIQUE KEY uk_active_slot (active_slot);
