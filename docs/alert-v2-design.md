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

### 2.4 目标选择

`targets` 存 JSON 选择器，支持多选/分组/排除：

```json
{"include": {"agents": [8, 9], "agent_groups": ["nas"], "probes": [3]}, "exclude": {"agents": [10]}}
```

前端必须给图形选择器（多选 + 全选 + 排除），不再只能「全部机器」。

### 2.5 通知：渠道 + 路由 + 聚合

新表：
- `alert_channels`（type: wechat_work / dingtalk / webhook / email，name，config JSON，enabled）
- `alert_routes`（匹配 severity/rule labels → channel_ids，group_wait_s，group_interval_s，repeat_interval_s）
- `alert_silences`（matcher JSON，starts_at，ends_at，reason）
- `webhook_outbox`（**重试队列落库**：payload，attempts，next_retry_at，status，last_error）→ 解 P0#3

Dispatcher 行为：
- **聚合**：`group_wait`（默认 30s）内同一规则的多目标合并成一条「fnOS、UNRAID、PVE 共 3 台离线」
- **恢复通知**：默认只在「组内全部恢复」时发一条，而不是每台各发一条
- **静默**：状态照常推进（still firing），但不发送，`suppressed_count++`
- **冷却**：拆成 `min_retrigger_after_resolve_s`（默认 60s，只防抖），不再用 10 分钟压真故障

### 2.6 面板

- 规则向导：选指标 →（存活型免填算子）→ 选目标（多选/分组/排除）→ **实时显示选中目标当前值和会不会触发** → 选通知渠道
- 规则详情页：每个目标的 `当前值 / 状态 / 已持续时长 / 通知次数`
- 事件页：ack / 一键静默 2h / 查看原始 value
- 首页顶部：告警引擎健康条（最后评估时间、失败通知数）

### 2.7 自监控

`GET /api/v1/alerts/health` → `{last_eval_at, eval_duration_ms, rules, targets, firing, outbox_pending, outbox_failed, notify_success_1h}`。
再加一条内置规则：**告警引擎自己超过 3 个周期没评估 → 用现存的最后一个渠道发一条**（dead-man's switch）。

---

## 三、迁移路径

1. **DB migration（schema v?? → +1）**
   - 建新表；`alert_rules` 增列 `kind`、`for_s`、`targets`、`labels`（旧的 `scope_type/scope_id/metric_type/duration_s` 保留一版做兼容读写）
   - 旧规则转换：`agent_offline` → `kind=availability, for_s=threshold*60`；其余 → `kind=threshold, for_s=duration_s`，`targets.include` 由 scope 推出
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
9. **多目标选择**：只包含 2 台 → 另外 2 台离线不告警
10. **引擎自监控**：kill 评估循环 → 超 3 周期后 dead-man 告警

---

## 五、明天执行顺序（每步可独立交付 + 可回滚）

| 步 | 内容 | 预估 |
|---|---|---|
| 1 | Provider/Sample 抽象重构，行为不变，10 条用例里 1/2/9 先绿 | 2h |
| 2 | `alert_rule_state` 落库 + `first_true_at` 语义 + migration（解 P0#1、P1#6） | 3h |
| 3 | 渠道/路由/outbox 重试/聚合/静默（解 P0#3#4、P1#8） | 4h |
| 4 | 前端规则向导 + 事件页 ack/静默 + 健康条 | 3h |
| 5 | 灰度开关验证 + 文档 + 打 tag | 1h |

**待 Tony 确认的决策点**
- A. `alert_rules` 表结构可以改（需迁移旧规则），还是要保持现有 API 100% 兼容？
- B. 通知渠道这次只做企业微信，还是顺带把钉钉/通用 webhook 一起做？
- C. 聚合窗口默认 30s 够不够（意味着告警最快晚 30 秒到手机上）？
