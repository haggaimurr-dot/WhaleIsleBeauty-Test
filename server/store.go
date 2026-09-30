package main

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

// 存储层。线上用 MySQL（store_mysql.go），本地开发和测试用内存（这个文件）。
// 业务规则都在 app.go，存储层只负责读写和“同一时段只能有一个进行中的预约”这条硬约束。

var (
	ErrNotFound  = errors.New("not found")
	ErrSlotTaken = errors.New("slot taken")
	// ErrStale：写入时预约状态已经被别的请求改了
	ErrStale = errors.New("stale")
)

type User struct {
	ID        string
	OpenID    string
	Nickname  string
	Avatar    string
	Skin      SkinProfile // UpdatedAt 为空表示从没保存过
	CreatedAt time.Time
}

// BookingRow 是存储里的预约。时间都用 UTC 存
type BookingRow struct {
	ID           string
	UserID       string
	ServiceID    string
	ServiceName  string
	ArtistID     string
	ArtistName   string
	Date         string
	Time         string
	StartAt      time.Time
	EndAt        time.Time
	DurationMin  int
	Price        int
	Deposit      int
	Status       string
	Occasion     string
	SkinType     string
	Note         string
	CreatedAt    time.Time
	PayDeadline  *time.Time
	PaidAt       *time.Time
	RefundedAt   *time.Time
	CancelReason string
}

func slotKey(artistID, date, t string) string { return artistID + "|" + date + "|" + t }

type Store interface {
	// UserByOpenID 找不到返回 nil, nil
	UserByOpenID(ctx context.Context, openid string) (*User, error)
	UsersByIDs(ctx context.Context, ids []string) (map[string]*User, error)
	// CreateUser 同一个 openid 并发创建时，后到的返回已存在的那个
	CreateUser(ctx context.Context, u *User) (*User, error)
	SaveSkinProfile(ctx context.Context, userID string, p SkinProfile) error
	CountCompleted(ctx context.Context, userID string) (int, error)

	// InsertBooking 同一 (artist, date, time) 已有进行中的预约时返回 ErrSlotTaken
	InsertBooking(ctx context.Context, b *BookingRow) error
	Booking(ctx context.Context, id string) (*BookingRow, error)
	// UpdateBooking 只在库里的状态仍是 from 时写入，否则返回 ErrStale；改到的时段被占用返回 ErrSlotTaken
	UpdateBooking(ctx context.Context, b *BookingRow, from string) error
	BookingsByUser(ctx context.Context, userID string, statuses []string) ([]*BookingRow, error)
	ActiveBookingsOn(ctx context.Context, date string) ([]*BookingRow, error)
	// DueBookings 返回需要自动流转的预约：付款超时、到点没确认、已确认且已结束
	DueBookings(ctx context.Context, now time.Time) ([]*BookingRow, error)

	// BlocksOn 返回这一天设了休息的时段，key 是 artistID|time
	BlocksOn(ctx context.Context, date string) (map[string]bool, error)
	AddBlock(ctx context.Context, artistID, date, t string) error
	RemoveBlock(ctx context.Context, artistID, date, t string) error
}

func isDue(b *BookingRow, now time.Time) bool {
	switch b.Status {
	case StatusPendingPayment:
		return b.PayDeadline != nil && !b.PayDeadline.After(now)
	case StatusPendingConfirm:
		return !b.StartAt.After(now)
	case StatusConfirmed:
		return !b.EndAt.After(now)
	}
	return false
}

// ---------- 内存实现 ----------

type memStore struct {
	mu       sync.Mutex
	users    map[string]*User // by id
	bookings map[string]*BookingRow
	blocks   map[string]bool // slotKey
}

func newMemStore() *memStore {
	return &memStore{users: map[string]*User{}, bookings: map[string]*BookingRow{}, blocks: map[string]bool{}}
}

func cloneBooking(b *BookingRow) *BookingRow { c := *b; return &c }
func cloneUser(u *User) *User               { c := *u; return &c }

func (s *memStore) UserByOpenID(_ context.Context, openid string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.OpenID == openid {
			return cloneUser(u), nil
		}
	}
	return nil, nil
}

func (s *memStore) UsersByIDs(_ context.Context, ids []string) (map[string]*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]*User{}
	for _, id := range ids {
		if u, ok := s.users[id]; ok {
			out[id] = cloneUser(u)
		}
	}
	return out, nil
}

func (s *memStore) CreateUser(_ context.Context, u *User) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.users {
		if x.OpenID == u.OpenID {
			return cloneUser(x), nil
		}
	}
	s.users[u.ID] = cloneUser(u)
	return cloneUser(u), nil
}

func (s *memStore) SaveSkinProfile(_ context.Context, userID string, p SkinProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.Skin = p
	return nil
}

func (s *memStore) CountCompleted(_ context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, b := range s.bookings {
		if b.UserID == userID && b.Status == StatusCompleted {
			n++
		}
	}
	return n, nil
}

// slotHolder 返回占着这个时段的进行中预约（除了 exceptID）
func (s *memStore) slotHolder(b *BookingRow, exceptID string) bool {
	for _, x := range s.bookings {
		if x.ID != exceptID && isActive(x.Status) && x.ArtistID == b.ArtistID && x.Date == b.Date && x.Time == b.Time {
			return true
		}
	}
	return false
}

func (s *memStore) InsertBooking(_ context.Context, b *BookingRow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if isActive(b.Status) && s.slotHolder(b, b.ID) {
		return ErrSlotTaken
	}
	s.bookings[b.ID] = cloneBooking(b)
	return nil
}

func (s *memStore) Booking(_ context.Context, id string) (*BookingRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bookings[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneBooking(b), nil
}

func (s *memStore) UpdateBooking(_ context.Context, b *BookingRow, from string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.bookings[b.ID]
	if !ok {
		return ErrNotFound
	}
	if cur.Status != from {
		return ErrStale
	}
	if isActive(b.Status) && s.slotHolder(b, b.ID) {
		return ErrSlotTaken
	}
	s.bookings[b.ID] = cloneBooking(b)
	return nil
}

func (s *memStore) filter(keep func(*BookingRow) bool) []*BookingRow {
	var out []*BookingRow
	for _, b := range s.bookings {
		if keep(b) {
			out = append(out, cloneBooking(b))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	return out
}

func (s *memStore) BookingsByUser(_ context.Context, userID string, statuses []string) ([]*BookingRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	want := set(statuses...)
	return s.filter(func(b *BookingRow) bool { return b.UserID == userID && want[b.Status] }), nil
}

func (s *memStore) ActiveBookingsOn(_ context.Context, date string) ([]*BookingRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filter(func(b *BookingRow) bool { return b.Date == date && isActive(b.Status) }), nil
}

func (s *memStore) DueBookings(_ context.Context, now time.Time) ([]*BookingRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filter(func(b *BookingRow) bool { return isDue(b, now) }), nil
}

func (s *memStore) BlocksOn(_ context.Context, date string) (map[string]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]bool{}
	for _, a := range artists {
		for _, t := range slotTimes {
			if s.blocks[slotKey(a.ID, date, t)] {
				out[a.ID+"|"+t] = true
			}
		}
	}
	return out, nil
}

func (s *memStore) AddBlock(_ context.Context, artistID, date, t string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocks[slotKey(artistID, date, t)] = true
	return nil
}

func (s *memStore) RemoveBlock(_ context.Context, artistID, date, t string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.blocks, slotKey(artistID, date, t))
	return nil
}
