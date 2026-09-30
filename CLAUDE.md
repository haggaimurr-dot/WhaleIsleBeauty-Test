# 鲸屿美妆小程序

私人化妆工作室的微信小程序。客人端用于看作品、预约、管理自己的预约；店主端用于排班和确认预约。两端在同一个小程序里，按用户角色显示入口。

当前阶段是**演示版**：所有数据来自本地 mock，不接真实后端、不接真实支付。后端将来用 Go 实现，部署在**微信云托管**，前端通过 `wx.cloud.callContainer` 调用（见 `src/api/http.ts`），接口契约已经定义在 `src/api/contract.ts`。

## 技术栈

- uni-app（Vue 3 + Vite + TypeScript），`<script setup lang="ts">`
- 目标平台只有微信小程序（mp-weixin）
- 样式用 SCSS，设计变量在 `src/styles/tokens.scss`
- 不引入 UI 组件库，所有组件自己写，保证视觉统一

## 常用命令

```bash
npm run dev:mp-weixin      # 开发，产物在 dist/dev/mp-weixin，用微信开发者工具导入
npm run build:mp-weixin    # 构建
npx vue-tsc --noEmit       # 类型检查，每完成一个任务都要跑一次并确保通过
```

## 设计参考

高保真原型在 `docs/prototype.html`，是所有页面的视觉和交互标准。实现页面前先读原型里对应的部分，尺寸、颜色、文案以原型为准。

原型按 375px 宽度设计，**所有尺寸换算成 rpx 时乘以 2**（原型 16px → 32rpx）。

## 视觉规则

- **颜色只用 `tokens.scss` 里的变量**，不要写死色值。
- **拱形是唯一的标志性图形**（上半圆、下圆角 20px）。用于人像、作品图、化妆师头像、成功页图标。其他卡片用普通圆角，不要到处都用拱形。
- **鼠尾草绿（$sage 系列）只用于两类信息**：安全感信息（卫生、价格透明、退款规则、隐私说明）和“已确认”状态。不要拿它做装饰。
- **不使用促销红、倒计时、“限时抢购”这类元素**，这是和美团页面刻意拉开的差别。
- 主按钮是摩卡色实心胶囊，次按钮是白底加细边框。一个页面底部最多一个主按钮。
- 标题用衬线字体栈 `$font-serif`，正文用系统字体。演示阶段不加载网络字体。
- 动效克制：只在预约成功（对勾描边）和底部弹层出现时使用动画，其他地方不加入场动画。
- **不显示滚动条，页面整体不滚动**：所有页面用 `PageLayout`，头尾固定（`#header` / `#footer`），只有中间内容滚动。pages.json 里每个页面都要设 `"disableScroll": true`。横向滑动的地方用 `scroll-view` 加 `enhanced` 和 `:show-scrollbar="false"`。

## 文案规则

- 语气像一位温和的化妆师在说话：口语、短句、不推销。例如“素颜过来就好”，而不是“欢迎莅临”。
- 按钮写清楚会发生什么：“付定金并预约”“预约同款”“取消预约”。同一个动作在整个流程里用同一个名字。
- 错误提示说明发生了什么、怎么办，不道歉、不含糊。例如“这个时间刚被约走了，换一个时间吧”。
- 空状态给出下一步，例如“还没有要来的预约”+「去预约」按钮。

## 目录结构

```
src/
  api/
    types.ts       # 所有数据类型，前后端共用的契约
    contract.ts    # Api 接口定义，注释里写了对应的 REST 路径
    mock.ts        # 内存 mock 实现
    http.ts        # 真实后端实现（将来接 Go 服务）
    pay.ts         # 定金支付流程封装
    client.ts      # USE_MOCK 开关和 api 实例
    index.ts       # 统一出口，页面只从这里 import
  utils/
    date.ts        # 日期格式化工具
    money.ts       # 金额格式化（分 → 元）
  styles/
    tokens.scss    # 设计变量
  components/      # 通用组件
  pages/           # 客人端页面
  pages-owner/     # 店主端分包
```

## 数据规则（重要）

- **页面只能通过 `import { api } from '@/api'` 获取数据**，禁止直接 import `mock.ts`。这样将来切换到真实后端时，页面代码不用改。
- 金额一律以**分**为单位的整数传递（`Cents` 类型），只在显示时用 `formatPrice()` 转换。
- 日期用 `'YYYY-MM-DD'` 字符串，时间用 `'HH:mm'` 字符串，格式化统一用 `utils/date.ts`。
- 所有 api 调用都可能抛出 `ApiError`，页面要根据 `code` 给出对应提示，至少处理 `SLOT_TAKEN` 和 `CANCEL_TOO_LATE`。
- 需要新接口时，先在 `types.ts` 和 `contract.ts` 里加定义，再在 `mock.ts` 里实现，最后才写页面。不要在页面里临时拼数据。

## 页面清单

| 路径 | 页面 | 原型对应 |
|---|---|---|
| pages/home/index | 首页 | 首页 |
| pages/works/index | 作品 | 作品 |
| pages/service/detail | 项目详情 | 项目详情 |
| pages/booking/index | 预约（tabBar 页，参数通过 `utils/tab.ts` 传入） | 预约 |
| pages/booking/reschedule | 改期（普通页面，query 带 `rescheduleId`，和预约页共用 `SlotPicker`） | 预约 |
| pages/booking/success | 预约成功 | 预约成功 |
| pages/me/index | 我的预约 | 我的预约 |
| pages/me/skin | 肤质档案（普通页面，从“我的”进入） | 无（原型只有入口，按预约页表单样式做） |
| pages-owner/schedule/index | 店主排班（分包） | 排班 |

TabBar 包含首页、作品、预约、我的，使用 `pages.json` 的原生 tabBar。图标需要 PNG 格式（81×81），放在 `static/tab/`。

## 通用组件（先做这些，再做页面）

- `ArchImage`：拱形图片。`src` 以 `placeholder:` 开头时显示渐变占位，例如 `placeholder:g2`，对应原型里的 g1–g6 渐变。
- `AppButton`：`variant="primary" | "ghost"`，`size="md" | "sm"`，支持 `disabled` 和 `loading`。
- `Chip`：可单选的标签，用于筛选和预约表单。
- `DayPicker`：横向日期选择，第一个显示“明天”。
- `StatusBadge`：根据 `BookingStatus` 显示对应颜色和文案。
- `PageLayout`：页面骨架，头尾固定、中间内容滚动、不显示滚动条。所有页面都用它。
- `BottomBar`：页面底部的操作栏，放在 `PageLayout` 的 `#footer` 里，自动处理 safe-area。

## 工作方式

- **一次只做一个页面或一个组件**，完成后停下来，说明做了什么、有哪些没做完。
- 完成后跑类型检查。不确定视觉是否正确时，告诉我在开发者工具里要检查哪里。
- 不要擅自新增依赖，需要时先说明理由。
- 不要修改 `src/api/types.ts` 和 `contract.ts` 里已有的字段名和含义，只能新增。这两个文件是和 Go 后端的契约。
