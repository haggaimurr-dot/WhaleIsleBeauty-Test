package main

import (
	"context"
	"time"
)

// 店主的经营统计。口径和 src/api/types.ts 的 MonthStats、mock.ts 的 monthStats 一致：
// 按到店日期归月，只算付过定金的预约；取消的定金都已退回，不算收入。

const statsMonths = 6

// MonthStats GET /owner/stats  最近 6 个月（含本月），本月在前
func (a *App) MonthStats(ctx context.Context, u *User) ([]MonthStats, error) {
	if err := a.requireOwner(u); err != nil {
		return nil, err
	}
	now := a.now().In(shanghai)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, shanghai)
	months := make([]string, statsMonths)
	for i := range months {
		months[i] = first.AddDate(0, -i, 0).Format("2006-01")
	}
	from := months[statsMonths-1] + "-01"
	to := first.AddDate(0, 1, 0).Format("2006-01-02")

	rows, err := a.store.PaidBookingsBetween(ctx, from, to)
	if err != nil {
		return nil, err
	}
	var userIDs []string
	seen := map[string]bool{}
	for _, b := range rows {
		if !seen[b.UserID] {
			seen[b.UserID] = true
			userIDs = append(userIDs, b.UserID)
		}
	}
	firstVisit, err := a.store.FirstVisits(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	byMonth := map[string]*MonthStats{}
	last := map[string]map[string]string{} // 月份 → 客人 → 这个月最后一次预约的日期
	out := make([]MonthStats, statsMonths)
	for i, m := range months {
		out[i].Month = m
		byMonth[m] = &out[i]
		last[m] = map[string]string{}
	}
	for _, b := range rows {
		m := b.Date[:7]
		s := byMonth[m]
		if s == nil || b.Status == StatusPendingPayment {
			continue
		}
		if b.Status == StatusCancelled {
			s.Cancelled++
			continue
		}
		s.Bookings++
		s.Deposit += b.Deposit
		if b.Date > last[m][b.UserID] {
			last[m][b.UserID] = b.Date
		}
	}
	for m, customers := range last {
		s := byMonth[m]
		s.Customers = len(customers)
		for id, date := range customers {
			if f := firstVisit[id]; f != "" && f < date {
				s.Returning++
			}
		}
	}
	return out, nil
}
