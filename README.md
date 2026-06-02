# Hawkeye - 个人系统监控

轻量级个人监控系统，Push 模式架构：Agent 主动上报指标，Server 被动接收。

## 架构

```
Agent (各机器) --HTTP POST--> Server --REST API--> React 仪表盘
                                      └--> 企业微信 Webhook (告警通知)
```

## 功能

- **系统监控**: CPU、内存、磁盘
- **网站探测**: HTTP 状态码检测、延迟监控、SSL 证书到期检测
- **告警通知**: 自定义阈值规则 + 企业微信 Webhook 推送
- **实时仪表盘**: React + Recharts 时序图表，WebSocket 实时更新
- **单二进制部署**: 前端嵌入 Go 二进制，一个文件搞定

## 技术栈

- **后端**: Go 1.23, Gin, modernc.org/sqlite (纯 Go, 无需 CGO)
- **前端**: React 19, TypeScript, Recharts, TailwindCSS
- **采集**: gopsutil/v4

## 一键安装

```bash
curl -fsSL https://www.icloud325.cn/sh/hawkeye-install.sh | bash
```

脚本会自动检测网络环境选择最快下载源（国内走阿里云，海外走 GitHub），
然后弹出交互式菜单：

```
请选择操作:
  1) 安装 Hawkeye
  2) 仅安装 Server
  3) 仅安装 Agent
  4) 卸载 Hawkeye
```

安装完成后自动创建系统服务（systemd / procd / Unraid go）并开机自启。

| 系统 | 服务管理 |
|------|----------|
| Ubuntu / Debian | systemd |
| OpenWrt | procd |
| Unraid | /boot/config/go |

### 卸载

在菜单中选择 `4) 卸载 Hawkeye`，脚本会自动检测已安装的组件，
停止所有服务、删除服务文件和安装目录，干净无残留。

## 手动编译

```bash
# 安装前端依赖
cd web && npm install && cd ..

# 构建前端
cd web && npm run build && cd ..

# 编译服务端（嵌入前端）
CGO_ENABLED=0 go build -tags embed -ldflags "-s -w -X main.version=1.3.0" -o bin/hawkeye-server ./cmd/server

# 编译 Agent (Linux amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=1.3.0" -o bin/hawkeye-agent ./cmd/agent
```

> **注意**: Windows 环境下 Go 的 `-ldflags` 在 PowerShell 中可能存在缓存问题，建议使用 Makefile 或 Python 脚本调用 Go 编译。

## 启动服务端

```bash
./bin/hawkeye-server --config ./server.yaml
```

服务端默认监听 `:18325`，打开浏览器访问即可。

## 启动 Agent

Agent 采集本地系统指标，主动推送到 Server：

```bash
./bin/hawkeye-agent --config ./agent.yaml
```

## 添加机器

1. 在 Web 界面「机器管理」页面创建 Agent，获取 Token
2. 在 Agent 配置文件中填写 Server 地址和 Token
3. 启动 Agent，自动上报指标

## 配置

### 服务端 (server.yaml)

```yaml
server:
  listen: ":18325"

database:
  path: "/opt/hawkeye/data/monitoring.db"
  retention_days: 30
  hourly_retention_days: 90

poller:
  offline_timeout_s: 90      # Agent 超过此时间无上报则标记离线
  check_interval_s: 30       # 离线检查间隔

alerts:
  wechat_webhook: ""
  check_interval_s: 30
```

### Agent (agent.yaml)

```yaml
server:
  listen: ":32518"            # Agent 自身端口（可选，调试用）
  url: "http://your-server:18325"  # Server 地址
  push_interval_s: 30         # 推送间隔

auth:
  token: "your-agent-token"   # 在 Server 前端创建 Agent 时生成

collect:
  temperature_enabled: true   # Linux 开启，Windows 关闭
  network_interfaces: []      # 空 = 全部网卡
```

## API

- `GET/POST/PUT/DELETE /api/v1/agents` — 机器管理
- `POST /api/v1/agents/push` — Agent 推送指标（无需认证）
- `GET /api/v1/agents/:id/metrics/:type` — 指标查询
- `GET/POST/PUT/DELETE /api/v1/probes` — 网站探测
- `GET/POST/PUT/DELETE /api/v1/alerts/rules` — 告警规则
- `GET /api/v1/alerts/events` — 告警历史
- `WS /ws` — WebSocket 实时推送

## 项目结构

```
├── cmd/server/          # 服务端入口
├── cmd/agent/           # Agent 入口
├── internal/
│   ├── models/          # 共享数据结构
│   ├── config/          # 配置加载
│   ├── agent/           # Agent 采集 + Push Reporter
│   ├── server/
│   │   ├── api/         # Gin 路由 + 处理器
│   │   ├── poller/      # 心跳检测
│   │   ├── storage/     # SQLite 存储
│   │   ├── alert/       # 告警引擎 + 通知
│   │   └── probe/       # HTTP 探测调度
│   └── static/          # 嵌入的前端构建产物
├── web/                 # React 前端源码
├── dist/                # 发布文件（二进制 + 安装脚本）
└── configs/             # 示例配置文件
```

## 部署目录结构

```
/opt/hawkeye/
├── hawkeye-server       # Server 二进制
├── hawkeye-agent        # Agent 二进制
├── server.yaml          # Server 配置
├── agent.yaml           # Agent 配置
├── data/
│   └── monitoring.db    # 数据库
├── logs/                # 日志
└── scripts/             # 脚本
```

## License

MIT
