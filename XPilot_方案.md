# XPilot 产品方案（Hackathon）

> 一句话：**把 Windows 变成游戏，把你的电脑使用过程变成 RPG。**

---

## 核心理念

不是监控，而是激励。

| 传统统计软件 | XPilot |
|-------------|--------|
| Chrome 用了 2 小时 | 🎉 今天获得 2 个成就 |
| 你摸鱼了 | 🔥 连续专注 95 分钟 |
| 冷冰冰的数字 | ⭐ 距离升级还差 120 XP |
| 系统定义规则 | **你自己设计成就** |

---

## 核心差异化（评委爆点）

### 🔥 用户自定义成就系统

这不是系统给你发成就，而是 **你自己设计成就**。

例如：
- 「连续 5 天不打开短视频软件」
- 「每天编码 2 小时，持续一周」
- 「每天学习英语 30 分钟」
- 「凌晨4点还在写代码」

软件自动判断是否达成，自动弹出通知，可生成分享卡片。

### 🎮 Windows Adventure（Windows 冒险）

你的电脑就是游戏世界：

| 软件 | 世界角色 |
|------|---------|
| VS Code / IDE | 🏛️ 程序员工会 |
| Chrome / Edge | 📚 图书馆 |
| WeChat / Discord | 🎪 社交广场 |
| 游戏 | ⚔️ 竞技场 |
| Zoom / Teams | 🏢 会议室 |
| 学习软件 | 🏋️ 训练场 |

整体 UI 像游戏世界地图，XP、等级、勋章、地图全部串联。

---

## 架构

```
┌─────────────────────────────────────────────────┐
│              Tauri App (Vue3 + TS)               │
│  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │
│  │Dashboard │ │World Map │ │ Achievement Wall │  │
│  │(Home/XP) │ │ (NPC Map)│ │  + Custom Editor │  │
│  ├──────────┤ ├──────────┤ └──────────────────┘  │
│  │  Stats   │ │ Profile  │ │    Settings       │  │
│  │(Charts)  │ │ (Level)  │ │    (Config)       │  │
│  └──────────┴─┴──────────┴─┴──────────────────┘  │
│                 Pinia Store / HTTP Client         │
└──────────────────────┬───────────────────────────┘
                       │ HTTP REST API (localhost)
┌──────────────────────┴───────────────────────────┐
│           Go Backend Service (System Tray)        │
│  ┌──────────────┐ ┌──────────┐ ┌─────────────┐  │
│  │ WindowTracker │ │ Analyzer │ │ Achievement  │  │
│  │ (每1s轮询)    │ │         │ │ Checker     │  │
│  ├──────────────┤ ├──────────┤ ├─────────────┤  │
│  │ SQLite Store │ │ XP/Level │ │ Notification │  │
│  └──────────────┘ └──────────┘ └─────────────┘  │
└──────────────────────────────────────────────────┘
```

### 关键流程

1. **Go 后台服务**每秒检查当前前台窗口（Windows API）
2. 记录进程名、窗口标题、开始/结束时间 → SQLite
3. **实时计算 XP**：编码 +10/min，阅读 +5/min，社交 +2/min，游戏 +2/min
4. **成就检查器**：每次数据更新后检查系统成就 + 用户自定义成就
5. 达成成就 → 通知前台弹出 Toast（Steam 风格）
6. **Tauri 前端**通过 HTTP API 拉取数据展示

> 用户关闭主窗口 → 仅关闭界面，Go 后台继续驻留系统托盘
> 真正退出 → 托盘菜单 → 退出程序

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面壳 | **Tauri v2** |
| 前端框架 | **Vue 3 + TypeScript** |
| 状态管理 | **Pinia** |
| CSS | **UnoCSS** |
| 图表 | **ECharts** |
| 后端 | **Go** |
| 数据库 | **SQLite** |
| Windows API | `GetForegroundWindow` / `GetWindowText` / `GetWindowThreadProcessId` |

---

## 数据库设计

```sql
-- 使用记录（每次切换窗口记录一条）
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    process_name TEXT NOT NULL,
    window_title TEXT,
    category TEXT DEFAULT 'other',
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration_seconds INTEGER DEFAULT 0,
    xp_earned REAL DEFAULT 0
);

-- 每日汇总
CREATE TABLE daily_stats (
    date TEXT PRIMARY KEY,
    total_xp REAL DEFAULT 0,
    total_minutes INTEGER DEFAULT 0,
    level INTEGER DEFAULT 1,
    streak_days INTEGER DEFAULT 0,
    achievements_unlocked INTEGER DEFAULT 0
);

-- XP 变动日志
CREATE TABLE xp_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    amount REAL NOT NULL,
    reason TEXT NOT NULL,
    session_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- 系统成就定义
CREATE TABLE achievements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    icon TEXT DEFAULT 'trophy',
    condition_type TEXT NOT NULL,
    condition_params TEXT NOT NULL,  -- JSON
    xp_reward INTEGER DEFAULT 50,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 用户自定义成就
CREATE TABLE custom_achievements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    icon TEXT DEFAULT 'trophy',
    condition_type TEXT NOT NULL,
    condition_params TEXT NOT NULL,  -- JSON
    xp_reward INTEGER DEFAULT 100,
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    color TEXT DEFAULT '#6EA8FE'
);

-- 已解锁成就（系统和自定义统一）
CREATE TABLE unlocked_achievements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,           -- 'system' or 'custom'
    source_id INTEGER NOT NULL,     -- achievements.id or custom_achievements.id
    unlocked_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 应用分类（NPC 映射）
CREATE TABLE app_categories (
    process_name TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    npc_name TEXT,
    icon TEXT
);

-- 用户设置
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- 初始系统成就
INSERT INTO achievements (name, description, icon, condition_type, condition_params, xp_reward) VALUES
('初出茅庐', '首次启动 XPilot', 'rocket', 'first_launch', '{}', 10),
('第一行代码', '首次打开编程工具', 'code', 'first_app', '{"category":"coding"}', 20),
('专注达人', '连续专注 90 分钟', 'flame', 'continuous_focus', '{"minutes":90}', 50),
('连胜7天', '连续签到 7 天', 'star', 'streak', '{"days":7}', 100),
('夜行者', '凌晨 2 点仍在学习/工作', 'moon', 'time_range', '{"start":"02:00","end":"04:00"}', 30),
('今日无游戏', '当天未启动任何游戏', 'shield', 'no_app', '{"category":"gaming"}', 30),
('Code Master', '连续编码 3 小时', 'zap', 'continuous_focus', '{"minutes":180,"category":"coding"}', 100),
('社交达人', '社交软件使用累计 1 小时', 'message', 'app_duration', '{"category":"social","minutes":60}', 30),
('探索者', '一天内使用 10 种不同应用', 'compass', 'app_count', '{"count":10}', 40),
('AI开发者', '连续 7 天使用编程工具', 'diamond', 'streak', '{"category":"coding","days":7}', 200),
('早起的虫', '早上 6 点前开始工作', 'sun', 'time_range', '{"start":"04:00","end":"06:00"}', 50),
('全栈工程师', '一天内同时使用了前端和后端工具', 'crown', 'app_combination', '{"apps":["code","vscode","webstorm","idea"]}', 80);
```

---

## XP & 等级系统

### XP 获取速率

| 活动 | XP/分钟 |
|------|---------|
| 🖥️ 编码（VS Code, IDE, Terminal） | +10 |
| 📖 阅读文档/学习 | +5 |
| 💬 社交（微信、Discord） | +2 |
| 🎮 游戏 | +2 |
| 💤 空闲/屏保 | 0 |
| 🏆 达成成就 | +奖励值 |
| ✍️ 自定义成就 | +自定义值 |

### 等级公式

```
升级所需 XP = 100 × 当前等级

等级  累计XP
Lv1   0
Lv2   100
Lv3   300
Lv4   600
Lv5   1000
Lv10  4500
Lv15  10500
Lv20  19000
Lv50  122500
```

### 等级称号

| 等级范围 | 称号 |
|---------|------|
| Lv1-5 | 🐣 初心者 |
| Lv6-10 | 🏃 冒险者 |
| Lv11-20 | ⚔️ 勇士 |
| Lv21-30 | 🛡️ 骑士 |
| Lv31-40 | 🌟 大师 |
| Lv41-50 | 👑 传说 |
| Lv51-70 | ✨ 史诗 |
| Lv71-100 | 🏆 神话 |

---

## 前端页面设计

### 1. Dashboard（首页）
```
┌──────────────────────────────────────────────────┐
│ 🎮 XPilot                          ⚙️  👤 Lv.15 │
├──────────────────────────────────────────────────┤
│                                                  │
│  ╔══════════════════════════════════════════════╗ │
│  ║  ⭐ Lv.15  程序员                  进度条   ║ │
│  ║                        ████████░░░░░  75%   ║ │
│  ║  🎉 今天已获得 2 个成就  🔥 连续专注95min  ║ │
│  ║  ⚡ XP: 10500 / 15000                       ║ │
│  ╚══════════════════════════════════════════════╝ │
│                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐         │
│  │ 📊 今日  │ │ 🏆 成就  │ │ 🌍 世界  │         │
│  │ 统计     │ │ 墙      │ │ 地图     │         │
│  └──────────┘ └──────────┘ └──────────┘         │
│                                                  │
│  当前活跃：VS Code · 已持续 1h23min              │
│  ┌──────────────────────────────────────┐        │
│  │ 今日时间线                           │        │
│  │ 09:00 ████████ VS Code               │        │
│  │ 09:35 ██ WeChat                      │        │
│  │ 09:40 ████████████ Chrome            │        │
│  │ 10:20 ████████ VS Code               │        │
│  │ 11:05 ██ Terminal                    │        │
│  └──────────────────────────────────────┘        │
└──────────────────────────────────────────────────┘
```

### 2. World Map（世界地图）
```
┌──────────────────────────────────────────────────┐
│ 🌍 Windows Adventure                   当前在线  │
├──────────────────────────────────────────────────┤
│                                                  │
│         ╔══════════════════╗                     │
│         ║   🏛️ 程序员工会   ║  ← 你在这里        │
│         ║  VS Code (35min) ║                     │
│         ╚══════════════════╝                     │
│                                                  │
│    ┌────────────┐         ┌────────────┐         │
│    │ 📚 图书馆   │         │ 🎪 社交广场 │         │
│    │ Chrome     │         │ WeChat    │         │
│    │ 今日: 2次  │         │ 今日: 1次 │         │
│    └────────────┘         └────────────┘         │
│                                                  │
│         ┌────────────┐         ┌────────────┐    │
│         │ ⚔️ 竞技场   │         │ 🏢 会议室   │    │
│         │ 今日: 0次  │         │ 今日: 0次  │    │
│         └────────────┘         └────────────┘    │
│                                                  │
│  📍 当前位置：程序员工会                         │
│  👥 NPC：VS Code 正在等你写代码                  │
└──────────────────────────────────────────────────┘
```

### 3. Achievement Wall（成就墙）
```
┌──────────────────────────────────────────────────┐
│ 🏆 成就墙                    [+ 创建自定义成就]  │
├──────────────────────────────────────────────────┤
│                                                  │
│  ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐            │
│  │ ⭐  │ │ 🔥  │ │ 🏆  │ │ 🔒  │ │ 🔒  │            │
│  │专注 │ │连胜 │ │Code │ │ ... │ │ ... │            │
│  │达人 │ │7天  │ │Mstr │ │     │ │     │            │
│  │✓    │ │✓    │ │✓    │ │     │ │     │            │
│  └────┘ └────┘ └────┘ └────┘ └────┘            │
│                                                  │
│  ─── 自定义成就 ───                              │
│  ┌────┐ ┌────┐                                   │
│  │ 💪  │ │ 🔒  │                                   │
│  │编码 │ │英语 │                                   │
│  │2h   │ │30m  │                                   │
│  │✓    │ │     │                                   │
│  └────┘ └────┘                                   │
└──────────────────────────────────────────────────┘
```

### 4. Custom Achievement Creator（自定义成就）
```
┌──────────────────────────────────────────────────┐
│ ✨ 创建自定义成就                                │
├──────────────────────────────────────────────────┤
│                                                  │
│  成就名称       [连续5天不刷短视频              ] │
│  描述           [坚持5天不打开抖音/快手         ] │
│  XP奖励         [200                           ] │
│  图标           [ 🎯 ▼ ]                        │
│  颜色           [ #6EA8FE ■ ]                   │
│                                                  │
│  条件类型       [ 连续天数 ▼ ]                   │
│  参数           [ 5 天 ]                         │
│  排除应用       [ douyin.exe, kuaishou.exe  ]    │
│                                                  │
│  ┌──────────────────────────────────────┐        │
│  │ 📱 预览卡片                           │        │
│  │ 🎯 连续5天不刷短视频                  │        │
│  │ 坚持5天不打开抖音/快手                │        │
│  │ +200 XP                               │        │
│  └──────────────────────────────────────┘        │
│                                                  │
│  [💾 保存]  [🔍 测试条件]                        │
└──────────────────────────────────────────────────┘
```

### 5. Stats（数据统计）
```
┌──────────────────────────────────────────────────┐
│ 📊 数据统计                    📅 2026-07-03     │
├──────────────────────────────────────────────────┤
│                                                  │
│  今日时间分配：                                   │
│  ┌──────────────────────────────────────────┐    │
│  │ ██████████████████ 编码 3h    ██████ 社交 │    │
│  │ ████████ 浏览 1.5h    ██ 其他   │    │
│  └──────────────────────────────────────────┘    │
│                                                  │
│  🔥 热力图（本周）                               │
│  ┌────┬────┬────┬────┬────┬────┬────┐           │
│  │ 一  │ 二  │ 三  │ 四  │ 五  │ 六  │ 日  │           │
│  │ ██  │ ███ │ ██  │ ████│ █   │    │    │           │
│  └────┴────┴────┴────┴────┴────┴────┘           │
│                                                  │
│  软件使用排行：                                   │
│  1. VS Code          ████████████  3h 20m        │
│  2. Chrome           ████████     1h 45m         │
│  3. WeChat           ███          45m            │
│  4. Terminal         ██           30m            │
│  5. Spotify          █            15m            │
└──────────────────────────────────────────────────┘
```

---

## Steam 风格成就弹窗

### 位置 & 动画
- 左下角，距离左侧 24px，距离底部 24px
- 队列依次弹出
- 底部向上滑入（250ms）
- 停留 3 秒
- 淡出并下滑（250ms）

### 尺寸 & 样式
- 360×96 px，圆角 16px
- 毛玻璃背景（18px Blur）
- 半透明深色背景（约 75%）
- 柔和阴影

### 布局
```
┌──────────────────────────────────────┐
│ 🏆  获得成就                          │
│                                      │
│ ⭐ 专注达人                           │
│ 连续专注 90 分钟                     │
│ +50 XP                     点击查看 › │
└──────────────────────────────────────┘
```

### 颜色
| 用途 | 色值 |
|------|------|
| 背景 | `#1F232A` |
| 主色 | `#6EA8FE` |
| 金色 | `#F7C948` |
| 正文 | `#FFFFFF` |
| 次级 | `#B8C0CC` |

### 图标类型
| 图标 | 用途 |
|------|------|
| 🏆 奖杯 | 默认 |
| ⭐ 星星 | 等级 |
| 🔥 火焰 | 连续 |
| ⚡ 闪电 | 速度 |
| 💎 钻石 | 稀有 |
| 👑 皇冠 | 史诗 |
| 💠 宝石 | 传说 |

---

## REST API 设计

### Go 后台暴露的本地 API

```
GET    /api/v1/stats/daily?date=2026-07-03    # 获取某日统计
GET    /api/v1/stats/range?start=&end=        # 获取范围统计
GET    /api/v1/stats/realtime                  # 获取当前活跃会话

GET    /api/v1/sessions?date=2026-07-03        # 获取当日使用记录
GET    /api/v1/sessions/current                # 获取当前会话

GET    /api/v1/achievements                    # 获取所有系统成就
GET    /api/v1/achievements/unlocked           # 获取已解锁成就

GET    /api/v1/custom-achievements             # 获取自定义成就
POST   /api/v1/custom-achievements             # 创建自定义成就
PUT    /api/v1/custom-achievements/:id         # 更新自定义成就
DELETE /api/v1/custom-achievements/:id         # 删除自定义成就

GET    /api/v1/profile                         # 获取玩家信息(等级/XP/称号)
GET    /api/v1/heatmap?days=7                  # 获取热力图数据

GET    /api/v1/apps/top?limit=10&date=         # 应用排行

POST   /api/v1/settings                        # 更新设置
GET    /api/v1/settings                        # 获取设置
```

---

## 开发计划

### Phase 1: Go 后台服务（核心引擎）
- [x] 项目结构 & 模块设计
- [ ] Windows API 窗口追踪（每秒轮询前台窗口）
- [ ] SQLite 数据库 & 数据层
- [ ] XP 计算引擎
- [ ] 系统成就检查器
- [ ] 用户自定义成就检查器
- [ ] HTTP API 服务器
- [ ] 系统托盘

### Phase 2: Tauri + Vue3 前端（用户界面）
- [ ] Tauri 项目初始化
- [ ] Vue3 + TS + Pinia + UnoCSS 配置
- [ ] Dashboard 首页（XP、等级、今日统计）
- [ ] 时间线视图（当日活动时间轴）
- [ ] World Map 世界地图
- [ ] Achievement Wall 成就墙
- [ ] Custom Achievement Creator 自定义成就编辑器
- [ ] Stats 统计页面（图表、热力图、排行）
- [ ] Profile 玩家信息
- [ ] Settings 设置页面

### Phase 3: 通知 & 集成
- [ ] Steam 风格成就弹窗组件
- [ ] 队列管理系统
- [ ] Go → Tauri 通知集成
- [ ] 成就分享卡片生成

### Phase 4: 打磨
- [ ] 毛玻璃 UI 主题完善
- [ ] 动画 & 过渡效果
- [ ] 性能优化（减少 Go 服务资源占用）
- [ ] 安装包构建

---

## 项目目录结构

```
XPilot/
├── src-tauri/              # Tauri 配置
│   ├── Cargo.toml
│   ├── tauri.conf.json
│   └── src/
│       └── main.rs
│
├── src/                    # Vue3 前端
│   ├── App.vue
│   ├── main.ts
│   ├── router/
│   │   └── index.ts
│   ├── stores/
│   │   ├── app.ts          # 全局状态
│   │   ├── stats.ts        # 统计数据
│   │   └── achievements.ts # 成就相关
│   ├── api/
│   │   └── index.ts        # HTTP API 封装
│   ├── components/
│   │   ├── SteamToast.vue  # 成就弹窗
│   │   ├── XpBar.vue       # XP 进度条
│   │   ├── LevelBadge.vue  # 等级徽章
│   │   └── ...             # 其他通用组件
│   ├── pages/
│   │   ├── Dashboard.vue   # 首页
│   │   ├── WorldMap.vue    # 世界地图
│   │   ├── Achievements.vue # 成就墙
│   │   ├── CustomAchievement.vue # 自定义成就
│   │   ├── Stats.vue       # 统计
│   │   ├── Profile.vue     # 玩家信息
│   │   └── Settings.vue    # 设置
│   └── styles/
│       └── main.css        # 全局样式
│
├── backend/                # Go 后台服务
│   ├── go.mod
│   ├── main.go
│   ├── tracker/
│   │   └── window.go       # Windows API 窗口追踪
│   ├── db/
│   │   └── sqlite.go       # 数据库操作
│   ├── api/
│   │   ├── router.go       # HTTP 路由
│   │   └── handlers.go     # 请求处理
│   ├── engine/
│   │   ├── xp.go           # XP 计算
│   │   ├── level.go        # 等级系统
│   │   └── achievement.go  # 成就检查
│   └── tray/
│       └── tray.go         # 系统托盘
│
├── package.json
├── vite.config.ts
├── tsconfig.json
├── uno.config.ts           # UnoCSS 配置
└── index.html
```