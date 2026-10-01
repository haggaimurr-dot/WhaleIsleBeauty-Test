package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySQL 实现。时间一律按 UTC 存 DATETIME。表结构在 migrations/ 里，启动时自动执行没跑过的。
// “同一时段只能有一个进行中的预约”靠 active_slot 唯一索引保证：它是生成列（见 002），
// 进行中时由数据库算出 artist|date|time，结束后为 NULL。代码里不要写这一列。

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
	if err := migrate(db, migrationFS); err != nil {
		db.Close()
		return nil, err
	}
	if err := seedCatalog(db); err != nil {
		db.Close()
		return nil, err
	}
	return &mysqlStore{db: db}, nil
}

// migrate 按文件名顺序执行 migrations/*.sql 里还没跑过的，每个文件只执行一次
func migrate(db *sql.DB, files fs.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(128) NOT NULL PRIMARY KEY,
		applied_at DATETIME     NOT NULL
	) DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	names, err := fs.Glob(files, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		// MySQL 的 DDL 会隐式提交，没法整体回滚：某个文件失败就停下，修好后重新部署会从这个文件继续
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("执行 %s 失败: %w", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, name, time.Now().UTC()); err != nil {
			return err
		}
		log.Printf("migration applied: %s", name)
	}
	return nil
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO bookings (`+bookingCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.UserID, b.ServiceID, b.ServiceName, b.ArtistID, b.ArtistName, b.Date, b.Time,
		b.StartAt.UTC(), b.EndAt.UTC(), b.DurationMin, b.Price, b.Deposit, b.Status, b.Occasion, b.SkinType, b.Note,
		b.CreatedAt.UTC(), nullTime(b.PayDeadline), nullTime(b.PaidAt), nullTime(b.RefundedAt), b.CancelReason)
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
		pay_deadline = ?, paid_at = ?, refunded_at = ?, cancel_reason = ?
		WHERE id = ? AND status = ?`,
		b.ArtistID, b.ArtistName, b.Date, b.Time, b.StartAt.UTC(), b.EndAt.UTC(), b.Status,
		nullTime(b.PayDeadline), nullTime(b.PaidAt), nullTime(b.RefundedAt), b.CancelReason,
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

// ---------- 订阅消息发送记录 ----------

func (s *mysqlStore) ClaimNotice(ctx context.Context, key, bookingID, kind string, at time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT IGNORE INTO notices (notice_key, booking_id, kind, result, created_at) VALUES (?, ?, ?, '', ?)`,
		key, bookingID, kind, at.UTC())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *mysqlStore) FinishNotice(ctx context.Context, key, result string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE notices SET result = ?, finished_at = ? WHERE notice_key = ?`, result, time.Now().UTC(), key)
	return err
}

func (s *mysqlStore) ReleaseNotice(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notices WHERE notice_key = ?`, key)
	return err
}

// ---------- 基础资料 ----------

// seedCatalog 表是空的时候写入初始数据。多个实例同时启动也没关系：主键冲突的被 IGNORE 掉
func seedCatalog(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM catalog`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now().UTC()
	for _, it := range seedCatalogItems() {
		if _, err := db.Exec(`INSERT IGNORE INTO catalog (kind, id, sort, data, updated_at) VALUES (?, ?, ?, ?, ?)`,
			it.Kind, it.ID, it.Sort, it.Data, now); err != nil {
			return fmt.Errorf("seed catalog: %w", err)
		}
	}
	log.Print("catalog seeded")
	return nil
}

func (s *mysqlStore) CatalogItems(ctx context.Context) ([]CatalogItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind, id, sort, data FROM catalog`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CatalogItem
	for rows.Next() {
		var it CatalogItem
		if err := rows.Scan(&it.Kind, &it.ID, &it.Sort, &it.Data); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *mysqlStore) PutCatalogItem(ctx context.Context, it CatalogItem) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO catalog (kind, id, sort, data, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE data = VALUES(data), updated_at = VALUES(updated_at)`,
		it.Kind, it.ID, it.Sort, it.Data, time.Now().UTC())
	return err
}

func (s *mysqlStore) DeleteCatalogItem(ctx context.Context, kind, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM catalog WHERE kind = ? AND id = ?`, kind, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *mysqlStore) ActiveBookingExists(ctx context.Context, artistID, serviceID string) (bool, error) {
	col, id := "artist_id", artistID
	if artistID == "" {
		col, id = "service_id", serviceID
	}
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bookings WHERE `+col+` = ? AND status IN (`+placeholders(len(activeStatuses))+`)`,
		append([]any{id}, toArgs(activeStatuses)...)...).Scan(&n)
	return n > 0, err
}
