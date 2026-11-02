# Hawkeye 告警系统 v2 重写清单

> 起因：2026-11-02 给四台机器加离线告警，做法是往指标体系里塞了个虚拟指标 `agent_offline`，
> 能用但很别扭。Tony 判断「这做法有点烂」，决定重写。
> 本文档 = 现状问题（带证据）+ 目标设计 + 迁移路径 + 验收用例，明天照着执行。

---

## 一、现状问题清单（按严重度）

### P0 — 会误事

| # | 问题 | 证据 | 后果 |
|---|---|---|---|
| 1 | **`duration_s` 计数在内存里，重启即清零** | `engine.go:234` `st.consecutive++`，`ruleState` 只在 `e.state` map | 离线 20 分钟，但 server 刚重启 → 再等满 `duration_s` 才报；宿主内存不稳导致服务频繁重开，这类漏报最危险 |
| 2 | **`cooldown_s` 从 resolved 时刻起算，会压掉真故障** | `engine.go:280-288` 判断 `time.Since(*lastEvent.ResolvedAt) < CooldownS` | 今天实测：13:09:56 恢复，13:12 再离线，600s 冷却把告警直接吞掉，我差点当成代码 bug 去查 |
| 3 | **通知失败只打日志，进程重启即丢** | `notifier.go` SendText 重试 3 次后 `log.Printf` | 网络抖动/企业微信限流期间的告警永久丢失，无补偿队列 |
| 4 | **无静默/维护窗口** | 无相关表与接口 | 换内存、重启宿主机、夜间批量重启 → 必然刷屏，久了就没人看告警 |

### P1 — 设计缺陷

| # | 问题 | 证据 |
|---|---|---|
| 5 | **指标靠字符串 + switch 硬编码，加一个指标改三处代码** | `engine.go:149` `if rule.MetricType == MetricAgentOffline` 特判；probe 侧 `switch rule.MetricType`；前端 `AlertConfig.tsx` 常量表 |
| 6 | **「离线」被塞进数值比较体系** | 值是「距 last_seen 的分钟数」，本质是 target 存活状态（dead-man's switch），跟着 `metrics` 的 `gt/lt` 语义走，单位也混（分钟 vs 百分比） |
| 7 | **目标选择只有一个 scope_type/scope_id** | `models.go:120` 附近；前端**没有选择器**，只能建「全部机器」的规则 |
| 8 | **无通知聚合** | 4 台同时掉线 = 4 条告警 + 4 条恢复 = 8 条消息，手机上一屏全是它 |
| 9 | **无 ack（认领）机制** | 事件只能 resolved 或一直挂着，不知道自己处理到哪 |
| 10 | **事件 message 是拼好的字符串，没有 value/fingerprint 字段** | 做 MTTR、抖动次数、趋势统计得解析中文文本 |

### P2 — 可观测性

| # | 问题 |
|---|---|
| 11 | 引擎自身无自监控：没有 `last_eval_at`、规则数、通知成功率，引擎崩了/卡了没人知道 |
| 12 | 状态机只存在内存 + 日志，排障只能看事件时间戳猜 |
| 13 | 两套时间阈值打架：`poller.offline_timeout_s: 90` 决定 status 翻转，`alerts.check_interval_s: 60` 决定评估频率，再叠规则阈值，实际延迟说不清 |

---

## 二、目标设计 v2

### 2.1 指标抽象：Provider 注册表

```go
type Sample struct {
    Metric string            // agent_up, agent_cpu, agent_mem, probe_status, cert_days_left...
    Value  float64
    Unit   string            // percent / minutes / ms / bool
    Labels map[string]string
}

type Provider interface {
    Name() string
    Samples(ctx context.Context, t Target) ([]Sample, error)  // 一次吐多个指标
}
```

内置 provider：
- `agent_up` — 由 `last_seen` 现算「离线分钟数」，语义是**存活探针**，不走数值比较
- `agent_stats` — cpu / memory / disk / load / net
- `probe` — status / latency / cert_days_left

新增指标 = 注册一个 provider，引擎、前端下拉框自动出现（后端提供 `GET /api/v1/alerts/metrics` 元数据：名称、单位、默认算子、默认阈值）。

### 2.1.1 两个入口：机器监控 / 网页监控，彻底分开

告警页一级导航拆成两个独立入口，**互不混装**：

| 入口 | 目标 | 典型规则 |
|---|---|---|
| 🖥 机器监控 | `agents`（PVE / UNRAID / fnOS / 阿里云…） | 离线、CPU、内存、磁盘、负载、网卡 |
| 🌐 网页监控 | `web_probes`（每个 URL） | 状态码不对、响应超时、证书快过期、证书过期 |

实现约束：
- `alert_rules` 增列 `target_class`（`agent` | `probe`），建规则时由入口决定，**一条规则只能挂同类目标**（加目标时强校验，跨类直接 400）
- 后端接口按入口隔离：
  - `GET/POST /api/v1/alerts/agent-rules`、`/api/v1/alerts/agent-rules/:id/targets`
  - `GET/POST /api/v1/alerts/probe-rules`、`/api/v1/alerts/probe-rules/:id/targets`
  - 事件/通知/健康/静默共用一套（事件里带 target_class 供筛选）
- 前端路由：`/alerts/machine`、`/alerts/web`，各自独立的规则列表 + 向导 + 目标表格 + 指标下拉
  （机器入口的指标下拉只出 cpu/memory/disk/load/离线，网页入口只出 status/latency/cert_days）
- 指标元数据接口带 `classes: ["agent"]` / `["probe"]`，下拉框自动过滤

这样点进哪个入口就只看得到哪类东西，不会在一堆 CPU 规则里找网站告警。

### 2.2 条件模型：Up/Down 与 Threshold 分开

- **存活型规则**（`kind = "availability"`）：只配 `for_s`（离线多久算故障），天然是 `down` 判定，不用填算子/阈值
- **阈值型规则**（`kind = "threshold"`）：metric + op + threshold + `for_s`

这样「离线 5 分钟」= `kind=availability, for_s=300`，不用再解释「阈值 5 分钟，持续填 0」。

### 2.3 状态持久化（P0 修复的核心）

新表 `alert_rule_state`：

| 列 | 说明 |
|---|---|
| rule_id, target_type, target_id | 唯一键，替代内存 `stateKey` |
| status | ok / pending / firing / silenced |
| first_true_at | **条件首次成立的时间戳**（wall clock），`for_s` 用它算，重启不丢、精度不受 60s 周期影响 |
| last_eval_at, last_value | 排障/面板直接展示「当前值 + 已持续多久」 |
| fired_at, last_notify_at, notify_count, suppressed_count | 去重、重复提醒、被冷却抑制次数 |
| UNIQUE(rule_id, target_type, target_id) | |

`for_s` 语义改为：`now - first_true_at >= for_s`（不再用 `consecutive × interval`）。
条件一旦不成立 → `first_true_at` 清空（正确的「连续」语义）。

### 2.4 目标配置：每台机器、每个网站各自一份（Tony 明确要求）

**不允许「一条规则套住全部对象」这种糊在一起的写法。** 规则只是逻辑容器，真正被评估的是
「规则 × 目标」这一行，**每行的算子、阈值、持续时长、开关、静默都可以单独设**。

新表 `alert_rule_targets`：

| 列 | 说明 |
|---|---|
| rule_id | 所属规则（只管通知渠道、重复提醒这类共享设置） |
| target_type | agent（电脑/NAS） / probe（网站） |
| target_id | agents.id / web_probes.id |
| enabled | 这台是否启用（默认继承规则，可单独关） |
| operator | gt / gte / lt / lte / eq / neq（**可覆盖**，存活型规则此列忽略） |
| threshold | 数值阈值（**可覆盖**，每台机器可以不一样） |
| for_s | 持续多久才告警（**可覆盖**，每台机器可以不一样） |
| silenced_until | 单台临时静默到期时间 |
| labels | 额外标签（如 `env=home`） |
| UNIQUE(rule_id, target_type, target_id) | |

后端 API：

```
GET    /api/v1/alerts/rules/:id/targets            # 目标清单 + 当前值 + 当前状态
POST   /api/v1/alerts/rules/:id/targets            # 追加目标（可一次多台，各自默认值）
PATCH  /api/v1/alerts/rules/:id/targets/:tid       # 单独改这一台的阈值/持续/开关
DELETE /api/v1/alerts/rules/:id/targets/:tid
POST   /api/v1/alerts/rules/:id/targets/bulk       # 批量应用（「把 5 分钟应用到全部」按钮）
```

前端形态（重点是「一眼看到每台的配置」）：

```
规则：机器离线         通知：企业微信-运维群   冷却：60s   重复提醒：30min
┌───────────────┬──────┬──────────┬────────┬────────┬──────────────┐
│ 目标          │ 当前 │ 离线时长 │ 阈值   │ 持续   │ 状态         │
├───────────────┼──────┼──────────┼────────┼────────┼──────────────┤
│ PVE-MS-03     │ ●在线│    —     │ [5 ]分 │ [0 ]秒 │ 正常         │
│ UNRAID        │ ●在线│    —     │ [5 ]分 │ [0 ]秒 │ 正常         │
│ fnOS          │ ○离线│  3 分钟  │ [3 ]分 │ [0 ]秒 │ pending 3/3  │
│ 阿里云        │ ●在线│    —     │ [10]分 │ [0 ]秒 │ 正常         │
└───────────────┴──────┴──────────┴────────┴────────┴──────────────┘
网站监控同理：每个网站一行，状态码/响应时间/证书天数各自设阈值。
```

要点：
- 新增机器/网站**不会**被静默套进现有规则——必须显式加入（避免「忘了配却以为有告警」）
- 但向导提供「全选当前所有机器/网站」一键加入，加入后每行仍可单独改
- 事件、状态、通知全部按 `(rule, target)` 独立，互不影响（今天的 `stateKey` 方向是对的，这里补上配置层的独立）

### 2.5 通知：不搞渠道表，就一条 webhook

单用户自用，**不建 `alert_channels`、不建 `alert_routes`、不建 `alert_silences` 表**。
通知地址沿用现有两处，够用：

1. `settings.default_wechat_webhook` — 全局默认（现在线上就是它，企业微信运维机器人）
2. `alert_rules.wechat_webhook` — 单条规则可选覆盖（留空 = 用全局）

聚合/冷却这类参数直接是规则字段，不另建路由表：

| 字段 | 含义 | 默认 |
|---|---|---|
| `group_wait_s` | 多目标同时告警的合并窗口 | 30 |
| `repeat_interval_s` | 持续未恢复时的重复提醒间隔 | 1800 |
| `min_retrigger_after_resolve_s` | 恢复后多久才允许再报（只防抖） | 60 |

静默：只做字段，不做表 —— `alert_rules.silenced_until`（整条规则静默）、
`alert_rule_targets.silenced_until`（单台/单个网站静默），面板上「静默 2 小时」按钮就是写这个字段。

**唯一保留的表：`webhook_outbox`（发送失败兜底队列）**
这不是渠道配置，是可靠性设施——解 P0#3「通知失败只打日志，进程重启即丢」：
发送失败 → 落库（payload, attempts, next_retry_at）→ 后台每 15s 重试 → 3 次仍失败标记 `failed`
并在 `/alerts/health` 暴露计数。企业微信群被解散、key 失效这类事发生时，你能在面板上看到，而不是永远不知道。

### 2.6 面板### 2.6 面板

- 两个入口各自独立的规则向导：选指标 →（存活型免填算子）→ 选目标（多选/分组/排除）→ **实时显示选中目标当前值和会不会触发** → 选通知渠道
- 规则详情页：每个目标的 `当前值 / 状态 / 已持续时长 / 通知次数`
- 事件页：ack / 一键静默 2h / 查看原始 value
- 首页顶部：告警引擎健康条（最后评估时间、失败通知数）

### 2.7 自监控

`GET /api/v1/alerts/health` → `{last_eval_at, eval_duration_ms, rules, targets, firing, outbox_pending, outbox_failed, notify_success_1h}`。
再加一条内置规则：**告警引擎自己超过 3 个周期没评估 → 用现存的最后一个渠道发一条**（dead-man's switch）。

---

## 三、迁移路径

1. **DB migration（schema v?? → +1）**
   - 建新表 `alert_rule_state` / `alert_rule_targets` / `webhook_outbox`（渠道/路由/静默表一律不建）
   - `alert_rules` 增列 `kind`、`target_class`、`labels`；旧的 `scope_type/scope_id/metric_type/operator/threshold/duration_s` 保留一版只读兼容
   - 旧规则转换：按 `metric_type` 判定 `target_class`（probe_* / cert_* → probe，其余 → agent）；scope 为 null 的规则**展开成每个目标一行**（`alert_rule_targets`），参数从规则原有字段复制 → 语义不变但从此可以逐台调
   - `agent_offline` → `kind=availability`，`for_s = threshold*60`；其余 → `kind=threshold`，`for_s = duration_s`
   - 旧的事件表结构不动（`message` 保留），新增列 `value`、`fingerprint` 可空
2. **API 双写/双读一个版本**：新 `POST /alerts/rules` 接受 `kind/targets`；旧字段仍能提交（转换后落库）。
3. **引擎只跑新模型**，provider 层保证行为等价（现有 4 条用例必须全绿）。
4. **灰度**：`alerts.v2: true` 开关，默认 off；验证期两条链路结果对比日志（`[alert-eval] v1 fired=… v2 fired=…`），一致后再切。
5. 回滚：`alerts.v2: false` + 保留 `.bak` 二进制。

---

## 四、验收用例（明天写，自动化）

1. 阈值型：CPU 95 > 90 → 触发 1 次；再次评估不重复；回落 → resolve
2. 存活型：`last_seen` 回拨 6 分钟，`for_s=300` → 触发；消息含「已离线 6 分钟」
3. **重启不丢**：把 state 造到 `first_true_at = now-20min`，重启 server → 第一个周期立刻 firing（不重新计时）
4. **连续语义**：成立 200s → 断开 10s → 再成立 200s，`for_s=300` 时不得触发（`first_true_at` 被清空）
5. **聚合**：3 台同时离线 → `group_wait` 内只发 1 条，且文字列出全部机器名
6. **静默**：静默窗口内 firing 但不发送，`suppressed_count` 增加；窗口结束仍 firing → 补发 1 条
7. **通知失败恢复**：webhook 返回 `errcode 93000` → outbox 进入重试，3 次失败后 `failed` 且 health 暴露计数
8. **冷却只防抖**：resolve 后 30s 内再触发被抑制；超过 60s 立刻告警（不再有 10 分钟黑洞）
9. **逐台独立**：同一规则下 fnOS 设 3 分钟、阿里云设 10 分钟 → 两者离线 5 分钟时只有 fnOS 告警
10. **网站独立**：3 个网站各自不同响应时间阈值，只有一个超阈值时只报那一个
11. **新目标不自动加入**：新增第 5 台机器，不加入规则 → 离线 1 小时也不告警（并在面板提示「未纳入告警」）
12. **单台静默**：只静默 fnOS 2 小时，其它机器照常告警
13. **入口隔离**：机器入口建的规则不能添加网站目标（400），反之亦然
14. **引擎自监控**：kill 评估循环 → 超 3 周期后 dead-man 告警

---

## 五、明天执行顺序（每步可独立交付 + 可回滚）

| 步 | 内容 | 预估 |
|---|---|---|
| 1 | Provider/Sample 抽象重构，行为不变，10 条用例里 1/2/9 先绿 | 2h |
| 2 | `alert_rule_state` 落库 + `first_true_at` 语义 + migration（解 P0#1、P1#6） | 3h |
| 3 | 渠道/路由/outbox 重试/聚合/静默（解 P0#3#4、P1#8） | 4h |
| 4 | 前端规则向导 + 事件页 ack/静默 + 健康条 | 3h |
| 5 | 灰度开关验证 + 文档 + 打 tag | 1h |

**已确认**
- 目标粒度：**每台电脑、每个网站分别配置**（2.4），不吃大锅饭
- 新机器/新网站**不自动纳入**规则，面板标「未纳入告警」，向导提供「全选当前全部」一键加入
- 告警页拆**两个入口**：机器监控 / 网页监控，规则与目标互不混装（2.1.1）

**默认决策（如需改再说）**
- 通知：**只做企业微信群机器人**，全局 settings 一条 webhook + 规则可选覆盖；不建渠道/路由/静默表
- 多目标聚合：默认开启，`group_wait = 30s`，可在路由上关闭
- 冷却：`min_retrigger_after_resolve_s = 60s`（只防抖，不再 10 分钟黑洞）
