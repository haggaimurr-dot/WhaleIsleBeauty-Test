package main

// 基础资料的初始数据，和前端 mock.ts 保持一致。第一次启动时写进 catalog 表，之后由店主在小程序里维护（catalog.go）。

var slotTimes = []string{"09:00", "10:30", "12:00", "13:30", "15:00", "16:30", "18:00", "19:30"}

var seedArtists = []Artist{
	{ID: "a1", Name: "小鲸", Title: "主理人", Years: 8, Avatar: "placeholder:g1"},
	{ID: "a2", Name: "安安", Years: 6, Specialty: "擅长新娘", Avatar: "placeholder:g4"},
	{ID: "a3", Name: "七七", Years: 4, Specialty: "擅长日常", Avatar: "placeholder:g3"},
}

var seedServices = []Service{
	{
		ID: "s1", Name: "韩式上镜妆（含发型）", Summary: "适合拍照、证件照", Category: "camera",
		DurationMin: 90, Price: 29800, Deposit: 5000,
		Includes: []string{"妆前护肤和底妆", "眼妆、修容、唇妆", "简单盘发或卷发", "假睫毛可选，不另收费"},
		Tags:     []string{"拍照不假面", "氧气感底妆", "含简单盘发"},
		Cover:    "placeholder:g2", Images: []string{"placeholder:g2", "placeholder:g6", "placeholder:g4"}, BookedCount: 128,
	},
	{
		ID: "s2", Name: "日常约会妆", Summary: "可教你日常怎么画", Category: "date",
		DurationMin: 60, Price: 16800, Deposit: 3000,
		Includes: []string{"妆前护肤和底妆", "眼妆和唇妆", "化妆师讲解日常画法"},
		Tags:     []string{"近看也干净", "可学可带走"},
		Cover:    "placeholder:g4", Images: []string{"placeholder:g4", "placeholder:g2"}, BookedCount: 96,
	},
	{
		ID: "s3", Name: "新娘跟妆", Summary: "需提前沟通，含试妆一次", Category: "bridal",
		DurationMin: 180, Price: 128000, PriceFrom: true, Deposit: 30000,
		Includes: []string{"试妆一次", "婚礼当天早妆", "全天跟妆补妆", "造型更换两次"},
		Tags:     []string{"持妆一整天", "含试妆"},
		Cover:    "placeholder:g3", Images: []string{"placeholder:g3", "placeholder:g1"}, BookedCount: 41,
	},
	{
		ID: "s4", Name: "主持/年会妆", Summary: "适合舞台灯光", Category: "host",
		DurationMin: 90, Price: 23800, Deposit: 5000,
		Includes: []string{"舞台底妆", "立体修容", "眼妆和唇妆", "简单发型"},
		Tags:     []string{"舞台灯光下", "上镜不反光"},
		Cover:    "placeholder:g5", Images: []string{"placeholder:g5"}, BookedCount: 57,
	},
}

var seedWorks = []Work{
	{ID: "w1", Title: "氧气上镜妆", Category: "camera", ArtistID: "a1", ServiceID: "s1", Image: "placeholder:g2", Ratio: 1.25, DurationText: "约 90 分钟"},
	{ID: "w2", Title: "清透约会妆", Category: "date", ArtistID: "a3", ServiceID: "s2", Image: "placeholder:g4", Ratio: 1.5, DurationText: "约 60 分钟"},
	{ID: "w3", Title: "中式新娘妆", Category: "bridal", ArtistID: "a2", ServiceID: "s3", Image: "placeholder:g3", Ratio: 1.6, DurationText: "需提前沟通"},
	{ID: "w4", Title: "年会主持妆", Category: "host", ArtistID: "a1", ServiceID: "s4", Image: "placeholder:g5", Ratio: 1.3, DurationText: "约 90 分钟"},
	{ID: "w5", Title: "毕业照妆", Category: "camera", ArtistID: "a3", ServiceID: "s1", Image: "placeholder:g6", Ratio: 1.45, DurationText: "约 75 分钟"},
	{ID: "w6", Title: "韩式新娘妆", Category: "bridal", ArtistID: "a2", ServiceID: "s3", Image: "placeholder:g1", Ratio: 1.25, DurationText: "需提前沟通"},
	{ID: "w7", Title: "面试淡妆", Category: "date", ArtistID: "a3", ServiceID: "s2", Image: "placeholder:g2", Ratio: 1.4, DurationText: "约 45 分钟"},
	{ID: "w8", Title: "证件照妆", Category: "camera", ArtistID: "a1", ServiceID: "s1", Image: "placeholder:g4", Ratio: 1.2, DurationText: "约 60 分钟"},
}

// 地址取自原型；电话和坐标是演示用的假数据
var seedShop = Shop{
	Name: "鲸屿美妆", Address: "蓝山CBD 3329", Phone: "020-0000-0000", OpenHours: "09:00–21:00",
	Latitude: 23.1291, Longitude: 113.2644,
}

func isSlotTime(t string) bool {
	for _, s := range slotTimes {
		if s == t {
			return true
		}
	}
	return false
}
