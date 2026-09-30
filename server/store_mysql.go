package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySQL 实现。时间一律按 UTC 存 DATETIME。
// “同一时段只能有一个进行中的预约”靠 active_slot 唯一索引保证：进行中时写 artist|date|time，结束后置 NULL。

const schema = `
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
`

type mysqlStore struct{ db *sql.DB }

// openMySQL 连上后自动建库建表
func openMySQL(addr, user, pass, dbName string) (*mysqlStore, error) {
	cfg := mysql.NewConfig()
	cfg.Net, cfg.Addr, cfg.User, cfg.Passwd = "tcp", addr, user, pass
	cfg.ParseTime, cfg.Loc = true, time.UTC
	cfg.MultiStatements = true
	cfg.ClientFoundRows = true // UPDATE 返回匹配的行数，值没变也算 1 行
	cfg.Params = map[string]string{"charset": "utf8mb4"}

	boot, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	_, err = boot.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "` DEFAULT CHARSET utf8mb4")
	boot.Close()
	if err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	cfg.DBName = dbName
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return &mysqlStore{db: db}, nil
}

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func timePtr(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time
	return &t
}

func activeSlot(b *BookingRow) sql.NullString {
	if !isActive(b.Status) {
		return sql.NullString{}
	}
	return sql.NullString{String: slotKey(b.ArtistID, b.Date, b.Time), Valid: true}
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

func toArgs(vs []string) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}

// ---------- 用户 ----------

const userCols = `id, openid, nickname, avatar, skin_type, tone, allergies, skin_note, skin_updated_at, created_at`

func scanUser(sc interface{ Scan(...any) error }) (*User, error) {
	var u User
	var updated sql.NullTime
	if err := sc.Scan(&u.ID, &u.OpenID, &u.Nickname, &u.Avatar, &u.Skin.SkinType, &u.Skin.Tone,
		&u.Skin.Allergies, &u.Skin.Note, &updated, &u.CreatedAt); err != nil {
		return nil, err
	}
	if updated.Valid {
		u.Skin.UpdatedAt = ts(updated.Time)
	}
	return &u, nil
}

func (s *mysqlStore) UserByOpenID(ctx context.Context, openid string) (*User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE openid = ?`, openid))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (s *mysqlStore) UsersByIDs(ctx context.Context, ids []string) (map[string]*User, error) {
	out := map[string]*User{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users WHERE id IN (`+placeholders(len(ids))+`)`, toArgs(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out[u.ID] = u
	}
	return out, rows.Err()
}

func (s *mysqlStore) CreateUser(ctx context.Context, u *User) (*User, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, openid, nickname, avatar, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.OpenID, u.Nickname, u.Avatar, u.CreatedAt.UTC())
	if isDuplicate(err) {
		return s.UserByOpenID(ctx, u.OpenID)
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *mysqlStore) SaveSkinProfile(ctx context.Context, userID string, p SkinProfile) error {
	var updated sql.NullTime
	if t, err := time.Parse(time.RFC3339, p.UpdatedAt); err == nil {
		updated = sql.NullTime{Time: t.UTC(), Valid: true}
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET skin_type = ?, tone = ?, allergies = ?, skin_note = ?, skin_updated_at = ? WHERE id = ?`,
		p.SkinType, p.Tone, p.Allergies, p.Note, updated, userID)
	return err
}

func (s *mysqlStore) CountCompleted(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE user_id = ? AND status = ?`, userID, StatusCompleted).Scan(&n)
	return n, err
}

// ---------- 预约 ----------

const bookingCols = `id, user_id, service_id, service_name, artist_id, artist_name, date, time, start_at, end_at,
  duration_min, price, deposit, status, occasion, skin_type, note, created_at, pay_deadline, paid_at, refunded_at, cancel_reason`

func scanBooking(sc interface{ Scan(...any) error }) (*BookingRow, error) {
	var b BookingRow
	var deadline, paid, refunded sql.NullTime
	if err := sc.Scan(&b.ID, &b.UserID, &b.ServiceID, &b.ServiceName, &b.ArtistID, &b.ArtistName, &b.Date, &b.Time,
		&b.StartAt, &b.EndAt, &b.DurationMin, &b.Price, &b.Deposit, &b.Status, &b.Occasion, &b.SkinType, &b.Note,
		&b.CreatedAt, &deadline, &paid, &refunded, &b.CancelReason); err != nil {
		return nil, err
	}
	b.PayDeadline, b.PaidAt, b.RefundedAt = timePtr(deadline), timePtr(paid), timePtr(refunded)
	return &b, nil
}

func (s *mysqlStore) queryBookings(ctx context.Context, where string, args ...any) ([]*BookingRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE `+where+` ORDER BY start_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*BookingRow
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *mysqlStore) InsertBooking(ctx context.Context, b *BookingRow) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO bookings (`+bookingCols+`, active_slot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.UserID, b.ServiceID, b.ServiceName, b.ArtistID, b.ArtistName, b.Date, b.Time,
		b.StartAt.UTC(), b.EndAt.UTC(), b.DurationMin, b.Price, b.Deposit, b.Status, b.Occasion, b.SkinType, b.Note,
		b.CreatedAt.UTC(), nullTime(b.PayDeadline), nullTime(b.PaidAt), nullTime(b.RefundedAt), b.CancelReason,
		activeSlot(b))
	if isDuplicate(err) {
		return ErrSlotTaken
	}
	return err
}

func (s *mysqlStore) Booking(ctx context.Context, id string) (*BookingRow, error) {
	b, err := scanBooking(s.db.QueryRowContext(ctx, `SELECT `+bookingCols+` FROM bookings WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (s *mysqlStore) UpdateBooking(ctx context.Context, b *BookingRow, from string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE bookings SET
		artist_id = ?, artist_name = ?, date = ?, time = ?, start_at = ?, end_at = ?, status = ?,
		pay_deadline = ?, paid_at = ?, refunded_at = ?, cancel_reason = ?, active_slot = ?
		WHERE id = ? AND status = ?`,
		b.ArtistID, b.ArtistName, b.Date, b.Time, b.StartAt.UTC(), b.EndAt.UTC(), b.Status,
		nullTime(b.PayDeadline), nullTime(b.PaidAt), nullTime(b.RefundedAt), b.CancelReason, activeSlot(b),
		b.ID, from)
	if isDuplicate(err) {
		return ErrSlotTaken
	}
	if err != nil {
		return err
	}
	// 开了 ClientFoundRows，0 行就是状态对不上
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStale
	}
	return nil
}

func (s *mysqlStore) BookingsByUser(ctx context.Context, userID string, statuses []string) ([]*BookingRow, error) {
	return s.queryBookings(ctx, `user_id = ? AND status IN (`+placeholders(len(statuses))+`)`,
		append([]any{userID}, toArgs(statuses)...)...)
}

func (s *mysqlStore) ActiveBookingsOn(ctx context.Context, date string) ([]*BookingRow, error) {
	return s.queryBookings(ctx, `date = ? AND active_slot IS NOT NULL`, date)
}

func (s *mysqlStore) DueBookings(ctx context.Context, now time.Time) ([]*BookingRow, error) {
	now = now.UTC()
	return s.queryBookings(ctx, `(status = ? AND pay_deadline <= ?) OR (status = ? AND start_at <= ?) OR (status = ? AND end_at <= ?)`,
		StatusPendingPayment, now, StatusPendingConfirm, now, StatusConfirmed, now)
}

// ---------- 休息时段 ----------

func (s *mysqlStore) BlocksOn(ctx context.Context, date string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT artist_id, time FROM blocks WHERE date = ?`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var artistID, t string
		if err := rows.Scan(&artistID, &t); err != nil {
			return nil, err
		}
		out[artistID+"|"+t] = true
	}
	return out, rows.Err()
}

func (s *mysqlStore) AddBlock(ctx context.Context, artistID, date, t string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT IGNORE INTO blocks (slot_key, artist_id, date, time, created_at) VALUES (?, ?, ?, ?, ?)`,
		slotKey(artistID, date, t), artistID, date, t, time.Now().UTC())
	return err
}

func (s *mysqlStore) RemoveBlock(ctx context.Context, artistID, date, t string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM blocks WHERE slot_key = ?`, slotKey(artistID, date, t))
	return err
}
