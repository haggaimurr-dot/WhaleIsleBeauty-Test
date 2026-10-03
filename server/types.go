package main

// 和前端 src/api/types.ts 一一对应的 JSON 结构。字段名、含义以 types.ts 为准，只能新增不能改。

type Artist struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Title     string `json:"title,omitempty"`
	Years     int    `json:"years"`
	Specialty string `json:"specialty,omitempty"`
	Avatar    string `json:"avatar"`
}

type Service struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Summary     string   `json:"summary"`
	Category    string   `json:"category"`
	DurationMin int      `json:"durationMin"`
	Price       int      `json:"price"`
	PriceFrom   bool     `json:"priceFrom,omitempty"`
	Deposit     int      `json:"deposit"`
	Includes    []string `json:"includes"`
	Tags        []string `json:"tags"`
	Cover       string   `json:"cover"`
	Images      []string `json:"images"`
	BookedCount int      `json:"bookedCount"`
	// Hidden 已下架：客人看不到、不能新约。只有店主接口和 GET /services/:id 会返回下架的项目
	Hidden bool `json:"hidden,omitempty"`
}

type Work struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Category     string  `json:"category"`
	ArtistID     string  `json:"artistId"`
	ServiceID    string  `json:"serviceId,omitempty"`
	Image        string  `json:"image"`
	Ratio        float64 `json:"ratio"`
	DurationText string  `json:"durationText"`
}

type Shop struct {
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	Phone     string  `json:"phone"`
	OpenHours string  `json:"openHours"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Me struct {
	ID          string `json:"id"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Role        string `json:"role"`
	VisitCount  int    `json:"visitCount"`
	SkinProfile string `json:"skinProfile,omitempty"`
}

type SkinProfile struct {
	SkinType  string `json:"skinType,omitempty"`
	Tone      string `json:"tone,omitempty"`
	Allergies string `json:"allergies,omitempty"`
	Note      string `json:"note,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type SlotView struct {
	Time      string `json:"time"`
	Available bool   `json:"available"`
}

type Booking struct {
	ID           string `json:"id"`
	ServiceID    string `json:"serviceId"`
	ServiceName  string `json:"serviceName"`
	ArtistID     string `json:"artistId"`
	ArtistName   string `json:"artistName"`
	Date         string `json:"date"`
	Time         string `json:"time"`
	DurationMin  int    `json:"durationMin"`
	Price        int    `json:"price"`
	Deposit      int    `json:"deposit"`
	Status       string `json:"status"`
	Occasion     string `json:"occasion,omitempty"`
	SkinType     string `json:"skinType,omitempty"`
	Note         string `json:"note,omitempty"`
	CreatedAt    string `json:"createdAt"`
	CanCancel    bool   `json:"canCancel"`
	PayDeadline  string `json:"payDeadline,omitempty"`
	CancelReason string `json:"cancelReason,omitempty"`
}

type CreateBookingReq struct {
	ServiceID string `json:"serviceId"`
	ArtistID  string `json:"artistId"`
	Date      string `json:"date"`
	Time      string `json:"time"`
	Occasion  string `json:"occasion"`
	SkinType  string `json:"skinType"`
	Note      string `json:"note"`
}

type WxPayParams struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

type CreateBookingResp struct {
	Booking Booking     `json:"booking"`
	Payment WxPayParams `json:"payment"`
}

// OwnerNotifyStatus 店主还能收几条预约变动提醒
type OwnerNotifyStatus struct {
	Quota int `json:"quota"`
}

type OwnerNotifyReq struct {
	Count int `json:"count"`
}

type SlotReq struct {
	ArtistID string `json:"artistId"`
	Date     string `json:"date"`
	Time     string `json:"time"`
}

type OwnerBookingBrief struct {
	ID           string       `json:"id"`
	CustomerName string       `json:"customerName"`
	ServiceName  string       `json:"serviceName"`
	Status       string       `json:"status"`
	Alert        string       `json:"alert,omitempty"`
	Note         string       `json:"note,omitempty"`
	CanConfirm   bool         `json:"canConfirm"`
	DurationMin  int          `json:"durationMin,omitempty"`
	Occasion     string       `json:"occasion,omitempty"`
	SkinType     string       `json:"skinType,omitempty"`
	Profile      *SkinProfile `json:"profile,omitempty"`
	// CanChange 店主能替客人改期、取消：进行中且还没开始
	CanChange bool `json:"canChange"`
	// CanMarkNoShow 店主能标记客人没来：已确认或已完成、开始时间已过、还是预约当天
	CanMarkNoShow bool `json:"canMarkNoShow"`
}

type ScheduleCell struct {
	ArtistID string             `json:"artistId"`
	Time     string             `json:"time"`
	State    string             `json:"state"`
	Booking  *OwnerBookingBrief `json:"booking,omitempty"`
}

type ArtistBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ScheduleStats struct {
	Total   int `json:"total"`
	Pending int `json:"pending"`
	Free    int `json:"free"`
	Stale   int `json:"stale"`
}

type DaySchedule struct {
	Date    string         `json:"date"`
	Times   []string       `json:"times"`
	Artists []ArtistBrief  `json:"artists"`
	Cells   []ScheduleCell `json:"cells"`
	Stats   ScheduleStats  `json:"stats"`
}

// ---------- 枚举 ----------

const (
	StatusPendingPayment = "pending_payment"
	StatusPendingConfirm = "pending_confirm"
	StatusConfirmed      = "confirmed"
	StatusCompleted      = "completed"
	StatusCancelled      = "cancelled"

	ReasonCustomer     = "customer"
	ReasonPayTimeout   = "pay_timeout"
	ReasonNotConfirmed = "not_confirmed"
	ReasonNoShow       = "no_show"
)

var (
	activeStatuses = []string{StatusPendingPayment, StatusPendingConfirm, StatusConfirmed}
	pastStatuses   = []string{StatusCompleted, StatusCancelled}

	occasions  = set("photo", "date", "interview", "event", "other")
	skinTypes  = set("dry", "oily", "combination", "sensitive", "unsure")
	skinTones  = set("cool_fair", "warm_fair", "natural", "wheat", "unsure")
	categories = set("camera", "date", "bridal", "host")
)

func set(vs ...string) map[string]bool {
	m := make(map[string]bool, len(vs))
	for _, v := range vs {
		m[v] = true
	}
	return m
}

func isActive(status string) bool {
	return status == StatusPendingPayment || status == StatusPendingConfirm || status == StatusConfirmed
}

// MonthStats 店主的月度汇总，口径见 src/api/types.ts
type MonthStats struct {
	Month     string `json:"month"`
	Bookings  int    `json:"bookings"`
	Cancelled int    `json:"cancelled"`
	NoShow    int    `json:"noShow"`
	Deposit   int    `json:"deposit"`
	Customers int    `json:"customers"`
	Returning int    `json:"returning"`
}
