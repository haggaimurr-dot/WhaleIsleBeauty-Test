package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// 基础资料：门店、化妆师、项目、作品。存在 catalog 表里，每条一行 JSON；第一次启动时写入 seed.go 的初始数据。
// 店主在小程序里维护，字段规则和前端 src/api/catalog.ts 一致，改的时候两边一起改。
// 已有预约存的是下单时的项目名、价格、定金、时长和化妆师名，改资料不影响已经下的单。

const (
	kindShop    = "shop"
	kindArtist  = "artist"
	kindService = "service"
	kindWork    = "work"
	shopID      = "shop"
)

// CatalogItem 存储里的一条资料。Sort 只在新建时写入：化妆师和项目按它升序，作品按它降序（最新的在前）
type CatalogItem struct {
	Kind string
	ID   string
	Sort int64
	Data []byte
}

type Catalog struct {
	Shop     Shop
	Artists  []Artist
	Services []Service // 含已下架的
	Works    []Work    // 最新的在前
}

func (c *Catalog) artist(id string) *Artist {
	for i := range c.Artists {
		if c.Artists[i].ID == id {
			return &c.Artists[i]
		}
	}
	return nil
}

func (c *Catalog) service(id string) *Service {
	for i := range c.Services {
		if c.Services[i].ID == id {
			return &c.Services[i]
		}
	}
	return nil
}

func item(kind, id string, sort int64, v any) CatalogItem {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err) // 都是本文件里的结构体，不会失败
	}
	return CatalogItem{Kind: kind, ID: id, Sort: sort, Data: data}
}

// seedCatalogItems 第一次启动时写进去的初始数据。作品的 sort 倒着给，保持 w1 在最前
func seedCatalogItems() []CatalogItem {
	out := []CatalogItem{item(kindShop, shopID, 0, seedShop)}
	for i, a := range seedArtists {
		out = append(out, item(kindArtist, a.ID, int64(i+1), a))
	}
	for i, s := range seedServices {
		out = append(out, item(kindService, s.ID, int64(i+1), s))
	}
	for i, w := range seedWorks {
		out = append(out, item(kindWork, w.ID, int64(len(seedWorks)-i), w))
	}
	return out
}

func buildCatalog(items []CatalogItem) (*Catalog, error) {
	sorted := append([]CatalogItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Sort != b.Sort {
			if a.Kind == kindWork {
				return a.Sort > b.Sort
			}
			return a.Sort < b.Sort
		}
		return a.ID < b.ID
	})
	c := &Catalog{Artists: []Artist{}, Services: []Service{}, Works: []Work{}}
	for _, it := range sorted {
		var err error
		switch it.Kind {
		case kindShop:
			err = json.Unmarshal(it.Data, &c.Shop)
		case kindArtist:
			var v Artist
			err = json.Unmarshal(it.Data, &v)
			v.ID = it.ID
			c.Artists = append(c.Artists, v)
		case kindService:
			var v Service
			err = json.Unmarshal(it.Data, &v)
			v.ID = it.ID
			c.Services = append(c.Services, v)
		case kindWork:
			var v Work
			err = json.Unmarshal(it.Data, &v)
			v.ID = it.ID
			c.Works = append(c.Works, v)
		}
		if err != nil {
			return nil, fmt.Errorf("catalog %s/%s: %w", it.Kind, it.ID, err)
		}
	}
	return c, nil
}

func (a *App) catalog(ctx context.Context) (*Catalog, error) {
	items, err := a.store.CatalogItems(ctx)
	if err != nil {
		return nil, err
	}
	return buildCatalog(items)
}

// ---------- 字段规则（对应 src/api/catalog.ts） ----------

const (
	limShopName     = 20
	limAddress      = 60
	limPhone        = 20
	limOpenHours    = 20
	limArtistName   = 10
	limArtistTitle  = 10
	limSpecialty    = 12
	limMaxYears     = 60
	limServiceName  = 20
	limSummary      = 30
	limDurationStep = 15
	limMinDuration  = 30
	limMaxDuration  = 480
	limMaxPrice     = 10_000_000 // 10 万元
	limIncludes     = 10
	limIncludeText  = 30
	limTags         = 5
	limTagText      = 10
	limImages       = 6
	limWorkTitle    = 20
	limDurationText = 20
	limMinRatio     = 0.5
	limMaxRatio     = 2
	limImageRef     = 255
)

var phonePattern = regexp.MustCompile(`^[0-9+\- ]+$`)

func runes(s string) int { return utf8.RuneCountInString(s) }

func textRule(v, label string, max int) string {
	if v == "" {
		return label + "还没填"
	}
	if runes(v) > max {
		return fmt.Sprintf("%s最多 %d 个字", label, max)
	}
	return ""
}

func optTextRule(v, label string, max int) string {
	if runes(v) > max {
		return fmt.Sprintf("%s最多 %d 个字", label, max)
	}
	return ""
}

func imageRule(v, label string) string {
	if v == "" {
		return "还没选" + label
	}
	if len(v) > limImageRef {
		return label + "地址太长了"
	}
	return ""
}

func firstRule(rules ...string) string {
	for _, r := range rules {
		if r != "" {
			return r
		}
	}
	return ""
}

// lines 去掉每行首尾空格，丢掉空行；结果不为 nil，JSON 里是 []
func lines(list []string) []string {
	out := []string{}
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func cleanShop(s Shop) Shop {
	s.Name, s.Address = strings.TrimSpace(s.Name), strings.TrimSpace(s.Address)
	s.Phone, s.OpenHours = strings.TrimSpace(s.Phone), strings.TrimSpace(s.OpenHours)
	return s
}

func validateShop(s Shop) string {
	phone := ""
	if s.Phone != "" && !phonePattern.MatchString(s.Phone) {
		phone = "电话只能写数字、空格、+ 和 -"
	}
	location := ""
	if math.IsNaN(s.Latitude) || math.IsNaN(s.Longitude) || math.Abs(s.Latitude) > 90 || math.Abs(s.Longitude) > 180 ||
		s.Latitude == 0 && s.Longitude == 0 {
		location = "地图位置还没选"
	}
	return firstRule(
		textRule(s.Name, "店名", limShopName),
		textRule(s.Address, "地址", limAddress),
		textRule(s.Phone, "电话", limPhone),
		phone,
		textRule(s.OpenHours, "营业时间", limOpenHours),
		location,
	)
}

func cleanArtist(v Artist) Artist {
	return Artist{
		Name: strings.TrimSpace(v.Name), Title: strings.TrimSpace(v.Title), Years: v.Years,
		Specialty: strings.TrimSpace(v.Specialty), Avatar: strings.TrimSpace(v.Avatar),
	}
}

func validateArtist(v Artist) string {
	years := ""
	if v.Years < 0 || v.Years > limMaxYears {
		years = fmt.Sprintf("从业年限填 0–%d 的整数", limMaxYears)
	}
	return firstRule(
		textRule(v.Name, "名字", limArtistName),
		optTextRule(v.Title, "头衔", limArtistTitle),
		optTextRule(v.Specialty, "擅长", limSpecialty),
		years,
		imageRule(v.Avatar, "头像"),
	)
}

// cleanService 封面就是第一张图；BookedCount 由服务端维护，这里清零，调用方再填
func cleanService(v Service) Service {
	images := lines(v.Images)
	cover := ""
	if len(images) > 0 {
		cover = images[0]
	}
	return Service{
		Name: strings.TrimSpace(v.Name), Summary: strings.TrimSpace(v.Summary), Category: v.Category,
		DurationMin: v.DurationMin, Price: v.Price, PriceFrom: v.PriceFrom, Deposit: v.Deposit,
		Includes: lines(v.Includes), Tags: lines(v.Tags), Cover: cover, Images: images, Hidden: v.Hidden,
	}
}

func validateService(v Service) string {
	var rules []string
	add := func(bad bool, msg string) {
		if bad {
			rules = append(rules, msg)
		}
	}
	rules = append(rules, textRule(v.Name, "项目名", limServiceName), textRule(v.Summary, "一句话介绍", limSummary))
	add(!categories[v.Category], "还没选风格")
	add(v.DurationMin < limMinDuration || v.DurationMin > limMaxDuration || v.DurationMin%limDurationStep != 0,
		fmt.Sprintf("时长在 %d–%d 分钟之间，按 %d 分钟一档", limMinDuration, limMaxDuration, limDurationStep))
	add(v.Price < 100 || v.Price > limMaxPrice, fmt.Sprintf("价格在 1–%d 元之间", limMaxPrice/100))
	add(v.Deposit < 100, "定金至少 1 元")
	add(v.Deposit > v.Price, "定金不能比价格高")
	add(len(v.Includes) > limIncludes, fmt.Sprintf("包含内容最多 %d 条", limIncludes))
	for _, s := range v.Includes {
		add(runes(s) > limIncludeText, fmt.Sprintf("包含内容每条最多 %d 个字", limIncludeText))
	}
	add(len(v.Tags) > limTags, fmt.Sprintf("标签最多 %d 个", limTags))
	for _, s := range v.Tags {
		add(runes(s) > limTagText, fmt.Sprintf("标签每个最多 %d 个字", limTagText))
	}
	add(len(v.Images) == 0, "至少选一张图")
	add(len(v.Images) > limImages, fmt.Sprintf("图片最多 %d 张", limImages))
	for _, s := range v.Images {
		rules = append(rules, imageRule(s, "图片"))
	}
	return firstRule(rules...)
}

func cleanWork(v Work) Work {
	return Work{
		Title: strings.TrimSpace(v.Title), Category: v.Category, ArtistID: v.ArtistID, ServiceID: v.ServiceID,
		Image: strings.TrimSpace(v.Image), Ratio: v.Ratio, DurationText: strings.TrimSpace(v.DurationText),
	}
}

func validateWork(v Work) string {
	category, artist, ratio := "", "", ""
	if !categories[v.Category] {
		category = "还没选风格"
	}
	if v.ArtistID == "" {
		artist = "还没选化妆师"
	}
	if math.IsNaN(v.Ratio) || v.Ratio < limMinRatio || v.Ratio > limMaxRatio {
		ratio = "图片比例不对"
	}
	return firstRule(
		textRule(v.Title, "标题", limWorkTitle),
		category, artist,
		imageRule(v.Image, "图片"),
		ratio,
		textRule(v.DurationText, "用时", limDurationText),
	)
}

// ---------- 客人端读取 ----------

func (a *App) Shop(ctx context.Context) (Shop, error) {
	c, err := a.catalog(ctx)
	if err != nil {
		return Shop{}, err
	}
	return c.Shop, nil
}

func (a *App) Artists(ctx context.Context) ([]Artist, error) {
	c, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	return c.Artists, nil
}

// Services 客人看到的项目，不含已下架的
func (a *App) Services(ctx context.Context) ([]Service, error) {
	c, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	out := []Service{}
	for _, s := range c.Services {
		if !s.Hidden {
			out = append(out, s)
		}
	}
	return out, nil
}

// Service 已下架的也返回，分享出去的详情页还能打开
func (a *App) Service(ctx context.Context, id string) (Service, error) {
	c, err := a.catalog(ctx)
	if err != nil {
		return Service{}, err
	}
	s := c.service(id)
	if s == nil {
		return Service{}, errNotFound
	}
	return *s, nil
}

// Works 客人看到的作品：关联的项目下架了就不带 serviceId，“预约同款”不预选
func (a *App) Works(ctx context.Context, category string) ([]Work, error) {
	c, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	out := []Work{}
	for _, w := range c.Works {
		if category != "" && w.Category != category {
			continue
		}
		if s := c.service(w.ServiceID); s != nil && s.Hidden {
			w.ServiceID = ""
		}
		out = append(out, w)
	}
	return out, nil
}

// ---------- 店主维护 ----------

// ownerCatalog 店主接口的公共开头：检查角色，读出当前资料
func (a *App) ownerCatalog(ctx context.Context, u *User) (*Catalog, error) {
	if err := a.requireOwner(u); err != nil {
		return nil, err
	}
	return a.catalog(ctx)
}

func (a *App) put(ctx context.Context, kind, id string, v any) error {
	return a.store.PutCatalogItem(ctx, item(kind, id, a.now().UnixMicro(), v))
}

func (a *App) remove(ctx context.Context, kind, id string) error {
	err := a.store.DeleteCatalogItem(ctx, kind, id)
	if errors.Is(err, ErrNotFound) {
		return errNotFound
	}
	return err
}

func (a *App) UpdateShop(ctx context.Context, u *User, req Shop) (Shop, error) {
	if _, err := a.ownerCatalog(ctx, u); err != nil {
		return Shop{}, err
	}
	s := cleanShop(req)
	if msg := validateShop(s); msg != "" {
		return Shop{}, badRequest(msg)
	}
	return s, a.put(ctx, kindShop, shopID, s)
}

func (a *App) saveArtist(ctx context.Context, u *User, id string, req Artist, isNew bool) (Artist, error) {
	c, err := a.ownerCatalog(ctx, u)
	if err != nil {
		return Artist{}, err
	}
	if !isNew && c.artist(id) == nil {
		return Artist{}, errNotFound
	}
	v := cleanArtist(req)
	if msg := validateArtist(v); msg != "" {
		return Artist{}, badRequest(msg)
	}
	v.ID = id
	return v, a.put(ctx, kindArtist, id, v)
}

func (a *App) CreateArtist(ctx context.Context, u *User, req Artist) (Artist, error) {
	return a.saveArtist(ctx, u, newID("a"), req, true)
}

func (a *App) UpdateArtist(ctx context.Context, u *User, id string, req Artist) (Artist, error) {
	return a.saveArtist(ctx, u, id, req, false)
}

func (a *App) DeleteArtist(ctx context.Context, u *User, id string) error {
	c, err := a.ownerCatalog(ctx, u)
	if err != nil {
		return err
	}
	if c.artist(id) == nil {
		return errNotFound
	}
	busy, err := a.store.ActiveBookingExists(ctx, id, "")
	if err != nil {
		return err
	}
	if busy {
		return apiErr("INVALID_STATE", "TA 还有没结束的预约，处理完再删")
	}
	n := 0
	for _, w := range c.Works {
		if w.ArtistID == id {
			n++
		}
	}
	if n > 0 {
		return apiErr("INVALID_STATE", fmt.Sprintf("TA 名下还有 %d 个作品，先改给别人或删掉", n))
	}
	if len(c.Artists) == 1 {
		return apiErr("INVALID_STATE", "至少要留一位化妆师")
	}
	return a.remove(ctx, kindArtist, id)
}

func (a *App) OwnerServices(ctx context.Context, u *User) ([]Service, error) {
	c, err := a.ownerCatalog(ctx, u)
	if err != nil {
		return nil, err
	}
	return c.Services, nil
}

// requireAnotherOnSale 下架或删掉 id 之后，至少还要有一个在接预约的项目，不然客人没法约
func requireAnotherOnSale(c *Catalog, id string) error {
	for _, s := range c.Services {
		if s.ID != id && !s.Hidden {
			return nil
		}
	}
	return apiErr("INVALID_STATE", "至少要留一个在接预约的项目")
}

func (a *App) CreateService(ctx context.Context, u *User, req Service) (Service, error) {
	if _, err := a.ownerCatalog(ctx, u); err != nil {
		return Service{}, err
	}
	v := cleanService(req)
	if msg := validateService(v); msg != "" {
		return Service{}, badRequest(msg)
	}
	v.ID = newID("s")
	return v, a.put(ctx, kindService, v.ID, v)
}

// UpdateService 改价只影响之后的新预约；已选择人数沿用原来的
func (a *App) UpdateService(ctx context.Context, u *User, id string, req Service) (Service, error) {
	c, err := a.ownerCatalog(ctx, u)
	if err != nil {
		return Service{}, err
	}
	old := c.service(id)
	if old == nil {
		return Service{}, errNotFound
	}
	v := cleanService(req)
	if msg := validateService(v); msg != "" {
		return Service{}, badRequest(msg)
	}
	if v.Hidden && !old.Hidden {
		if err := requireAnotherOnSale(c, id); err != nil {
			return Service{}, err
		}
	}
	v.ID, v.BookedCount = id, old.BookedCount
	return v, a.put(ctx, kindService, id, v)
}

// DeleteService 删掉后关联它的作品不再带 serviceId
func (a *App) DeleteService(ctx context.Context, u *User, id string) error {
	c, err := a.ownerCatalog(ctx, u)
	if err != nil {
		return err
	}
	s := c.service(id)
	if s == nil {
		return errNotFound
	}
	busy, err := a.store.ActiveBookingExists(ctx, "", id)
	if err != nil {
		return err
	}
	if busy {
		return apiErr("INVALID_STATE", "这个项目还有没结束的预约，可以先下架，等预约都结束了再删")
	}
	if !s.Hidden {
		if err := requireAnotherOnSale(c, id); err != nil {
			return err
		}
	}
	for _, w := range c.Works {
		if w.ServiceID == id {
			w.ServiceID = ""
			if err := a.store.PutCatalogItem(ctx, item(kindWork, w.ID, 0, w)); err != nil {
				return err
			}
		}
	}
	return a.remove(ctx, kindService, id)
}

func (a *App) OwnerWorks(ctx context.Context, u *User) ([]Work, error) {
	c, err := a.ownerCatalog(ctx, u)
	if err != nil {
		return nil, err
	}
	return c.Works, nil
}

func (a *App) saveWork(ctx context.Context, u *User, id string, req Work, isNew bool) (Work, error) {
	c, err := a.ownerCatalog(ctx, u)
	if err != nil {
		return Work{}, err
	}
	if !isNew {
		found := false
		for _, w := range c.Works {
			found = found || w.ID == id
		}
		if !found {
			return Work{}, errNotFound
		}
	}
	v := cleanWork(req)
	if msg := validateWork(v); msg != "" {
		return Work{}, badRequest(msg)
	}
	if c.artist(v.ArtistID) == nil {
		return Work{}, badRequest("选的化妆师不在了，换一位吧")
	}
	if v.ServiceID != "" && c.service(v.ServiceID) == nil {
		return Work{}, badRequest("关联的项目不在了，换一个吧")
	}
	v.ID = id
	return v, a.put(ctx, kindWork, id, v)
}

func (a *App) CreateWork(ctx context.Context, u *User, req Work) (Work, error) {
	return a.saveWork(ctx, u, newID("w"), req, true)
}

func (a *App) UpdateWork(ctx context.Context, u *User, id string, req Work) (Work, error) {
	return a.saveWork(ctx, u, id, req, false)
}

func (a *App) DeleteWork(ctx context.Context, u *User, id string) error {
	if _, err := a.ownerCatalog(ctx, u); err != nil {
		return err
	}
	return a.remove(ctx, kindWork, id)
}
