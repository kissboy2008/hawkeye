#!/bin/bash
# ============================================================
#  Hawkeye 一键安装脚本
#  支持: Server 端 / Agent 端 / Server+Agent 全套部署
#
#  用法:
#    交互模式:  curl -sSL https://raw.githubusercontent.com/xxx/hawkeye/main/scripts/install.sh | bash
#    参数模式:  bash install.sh --mode agent --token YOUR_TOKEN --server-url http://1.2.3.4:18325
#
#  参数:
#    --mode MODE           安装模式: server | agent | all（交互模式可跳过）
#    --port PORT           监听端口（server 默认 18325，agent 默认 32518）
#    --token TOKEN         Agent 认证 Token（agent 模式使用）
#    --server-url URL      Agent 上报的 Server 地址（agent 模式使用）
#    --dir DIR             安装目录（默认 /opt/hawkeye）
#    --temp-enabled        启用温度采集（仅 Linux，需 lm-sensors）
#    --no-start            安装后不启动服务
# ============================================================

set -euo pipefail

# ============================================================
# 可配置项（发布前修改）
# ============================================================
GITHUB_OWNER="kissboy2008"
GITHUB_REPO="hawkeye"
VERSION="v1.5.2"                               # 默认版本，可通过 --version 覆盖
# 下载源：优先从自定义源下载，GitHub Releases 作为默认
BINARY_BASE_URL="https://www.icloud325.cn/sh/hawkeye-bin/${VERSION}"
GITHUB_BASE_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${VERSION}"

# 安装目录（Server 和 Agent 共用）
INSTALL_DIR="/opt/hawkeye"

# 默认端口
SERVER_PORT="18325"
AGENT_PORT="32518"

# ============================================================
# 颜色 & 工具函数
# ============================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
step()  { echo -e "\n${BLUE}${BOLD}▶ $*${NC}"; }

# 交互输入（带默认值）
ask() {
    local prompt="$1"
    local default="$2"
    local var_ref="$3"

    if [[ -n "$default" ]]; then
        read -r -p "$(echo -e "${BLUE}→${NC} ${prompt} [${default}]: ")" input < /dev/tty
        input="${input:-$default}"
    else
        read -r -p "$(echo -e "${BLUE}→${NC} ${prompt}: ")" input < /dev/tty
    fi
    eval "$var_ref=\"$input\""
}

# ============================================================
# 系统检测
# ============================================================
detect_system() {
    # root 权限
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 权限运行: sudo bash $0"
    fi

    # 架构检测 (amd64 / arm64)
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        arm64)   ARCH="arm64" ;;
        *)
            warn "未测试的架构: $ARCH，尝试使用 amd64"
            ARCH="amd64"
            ;;
    esac
    info "系统架构: ${ARCH}"

    # OS 检测
    OS="linux"
    if [[ "$(uname -s)" == "Darwin" ]]; then
        OS="darwin"
    fi
}

# ============================================================
# 参数解析
# ============================================================
MODE=""
PORT=""
TOKEN=""
SERVER_URL=""
TEMP_ENABLED=false
NO_START=false
CUSTOM_DIR=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --mode)          MODE="$2"; shift 2 ;;
        --port)          PORT="$2"; shift 2 ;;
        --token)         TOKEN="$2"; shift 2 ;;
        --server-url)    SERVER_URL="$2"; shift 2 ;;
        --dir)           CUSTOM_DIR="$2"; shift 2 ;;
        --version)       VERSION="$2";
                         BINARY_BASE_URL="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${VERSION}";
                         shift 2 ;;
        --temp-enabled)  TEMP_ENABLED=true; shift ;;
        --no-start)      NO_START=true; shift ;;
        *)               warn "未知参数: $1"; shift ;;
    esac
done

# 自定义目录覆盖
if [[ -n "$CUSTOM_DIR" ]]; then
    INSTALL_DIR="$CUSTOM_DIR"
fi

# ============================================================
# 菜单选择
# ============================================================
choose_mode() {
    if [[ -n "$MODE" ]]; then
        case "$MODE" in
            server|agent|all) return ;;
            *) error "无效的 --mode 值: $MODE（可选: server / agent / all）" ;;
        esac
    fi

    echo ""
    echo -e "${BOLD}╔══════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║       🦅 Hawkeye 一键安装脚本       ║${NC}"
    echo -e "${BOLD}╠══════════════════════════════════════╣${NC}"
    echo -e "${BOLD}║                                      ║${NC}"
    echo -e "${BOLD}║${NC}  ${GREEN}1${NC}. 安装 Server 端（管理面板）     ${BOLD}║${NC}"
    echo -e "${BOLD}║${NC}  ${GREEN}2${NC}. 安装 Agent 端（监控采集）     ${BOLD}║${NC}"
    echo -e "${BOLD}║${NC}  ${GREEN}3${NC}. 全套安装（Server + Agent）    ${BOLD}║${NC}"
    echo -e "${BOLD}║${NC}  ${YELLOW}q${NC}. 退出                         ${BOLD}║${NC}"
    echo -e "${BOLD}║                                      ║${NC}"
    echo -e "${BOLD}╚══════════════════════════════════════╝${NC}"
    echo ""
    read -r -p "$(echo -e "${BLUE}请选择 [1-3]: ${NC}")" choice < /dev/tty

    case "$choice" in
        1) MODE="server" ;;
        2) MODE="agent" ;;
        3) MODE="all" ;;
        q|Q) echo "已取消"; exit 0 ;;
        *) error "无效的选择，请输入 1、2、3 或 q" ;;
    esac
}

# ============================================================
# 下载二进制
# ============================================================
download_binary() {
    local name="$1"       # hawkeye-server 或 hawkeye-agent
    local dest_dir="$2"
    local dest_file="$3"  # 最终文件名

    local download_url="${BINARY_BASE_URL}/${name}-${OS}-${ARCH}"
    local fallback_url="${GITHUB_BASE_URL}/${name}-${OS}-${ARCH}"
    local tmp_file="${dest_dir}/${name}.tmp"

    step "下载 ${name} ${VERSION} (${OS}/${ARCH})"
    mkdir -p "$dest_dir"

    # 尝试主下载源，失败后尝试 GitHub
    local downloaded=false
    for url in "$download_url" "$fallback_url"; do
        info "尝试下载: ${url}"
        if command -v curl >/dev/null 2>&1; then
            if curl -fSL --progress-bar "$url" -o "$tmp_file" 2>/dev/null; then
                downloaded=true
                break
            fi
        elif command -v wget >/dev/null 2>&1; then
            if wget -q --show-progress "$url" -O "$tmp_file" 2>/dev/null; then
                downloaded=true
                break
            fi
        fi
        warn "下载源 ${url} 失败，尝试下一个..."
    done

    if ! $downloaded; then
        rm -f "$tmp_file"
        error "下载失败！所有下载源均不可用:\n  1. ${download_url}\n  2. ${fallback_url}\n\n请检查:\n  - 网络是否通畅\n  - Releases 是否存在 ${VERSION}\n  - 或手动下载二进制到 ${dest_dir}"
    fi

    # 检查文件大小（至少 2MB）
    local file_size
    file_size=$(stat -c%s "$tmp_file" 2>/dev/null || stat -f%z "$tmp_file" 2>/dev/null || echo 0)
    if [[ "$file_size" -lt 2000000 ]]; then
        rm -f "$tmp_file"
        error "下载的二进制异常偏小 (${file_size} bytes)，请检查文件是否正确"
    fi

    mv "$tmp_file" "${dest_dir}/${dest_file}"
    chmod +x "${dest_dir}/${dest_file}"
    info "下载完成 (${file_size} bytes)"
}

# ============================================================
# 安装 Server
# ============================================================
install_server() {
    step "安装 Hawkeye Server (管理面板)"

    # 询问端口（交互模式下）
    local port="${PORT:-}"
    if [[ -z "$port" ]] && [[ "$MODE" != "all" || -z "$PORT" ]]; then
        ask "Server 监听端口" "$SERVER_PORT" port
    fi
    port="${port:-$SERVER_PORT}"

    local dir="$INSTALL_DIR"

    # 下载
    download_binary "hawkeye-server" "$dir" "hawkeye-server"

    # 生成 server.yaml
    step "生成 Server 配置文件"
    mkdir -p "${dir}/data" "${dir}/logs" "${dir}/scripts"

    cat > "${dir}/server.yaml" << EOF
# Hawkeye Server 配置
# 生成于: $(date '+%Y-%m-%d %H:%M:%S')

server:
  listen: ":${port}"
  mode: "release"

database:
  path: "${dir}/data/monitoring.db"
  retention_days: 30
  hourly_retention_days: 90

poller:
  interval_s: 30
  offline_timeout_s: 90

alerts:
  wechat_webhook: ""
  check_interval_s: 60

probes:
  check_interval_s: 60

auth:
  token: ""
EOF
    info "配置文件: ${dir}/server.yaml"
    info "数据目录:   ${dir}/data/"

    # 创建 systemd service
    create_systemd_service "hawkeye-server" \
        "Hawkeye Server - 管理面板" \
        "${dir}/hawkeye-server" \
        "-config ${dir}/server.yaml" \
        "$dir"

    # 启动
    start_service "hawkeye-server"

    echo ""
    info "Server 安装完成！"
    echo ""
    echo "  Web 管理面板:  http://$(hostname -I 2>/dev/null | awk '{print $1}'):${port}"
    echo "  配置文件:      ${dir}/server.yaml"
    echo "  数据目录:      ${dir}/data/"
    echo ""
}

# ============================================================
# 安装 Agent
# ============================================================
install_agent() {
    local auto_url="$1"  # 可选：全套安装时自动传入 Server URL

    step "安装 Hawkeye Agent (监控采集)"

    # 询问配置（交互模式 + 非自动模式）
    local token="${TOKEN:-}"
    local port="${PORT:-}"
    local server_url="${SERVER_URL:-$auto_url}"

    if [[ -z "$token" ]]; then
        ask "Agent 认证 Token（在 Server 面板「机器管理」中创建）" "" token
    fi
    if [[ -z "$port" ]]; then
        ask "Agent 监听端口" "$AGENT_PORT" port
    fi
    port="${port:-$AGENT_PORT}"

    if [[ -z "$server_url" ]]; then
        ask "Server 上报地址（如 http://1.2.3.4:18325）" "http://localhost:18325" server_url
    fi

    local dir="$INSTALL_DIR"

    # 下载
    download_binary "hawkeye-agent" "$dir" "hawkeye-agent"

    # 生成 agent.yaml
    step "生成 Agent 配置文件"
    mkdir -p "${dir}/data" "${dir}/logs" "${dir}/scripts"

    local temp_line="  temperature_enabled: false"
    if $TEMP_ENABLED; then
        temp_line="  temperature_enabled: true"
    fi

    cat > "${dir}/agent.yaml" << EOF
# Hawkeye Agent 配置
# 生成于: $(date '+%Y-%m-%d %H:%M:%S')

server:
  listen: ":${port}"
  url: "${server_url}"
  push_interval_s: 30

auth:
  token: "${token}"

collect:
${temp_line}
  network_interfaces: []
EOF
    info "配置文件: ${dir}/agent.yaml"

    # 创建 systemd service
    create_systemd_service "hawkeye-agent" \
        "Hawkeye Agent - 系统监控采集" \
        "${dir}/hawkeye-agent" \
        "-config ${dir}/agent.yaml" \
        "$dir"

    # 启动
    if [[ "$NO_START" != "true" ]]; then
        start_service "hawkeye-agent"
    fi

    echo ""
    info "Agent 安装完成！"
    echo ""
    echo "  安装目录:  $dir"
    echo "  二进制:    ${dir}/hawkeye-agent"
    echo "  配置文件:  ${dir}/agent.yaml"
    echo "  监听端口:  $port"
    echo "  上报地址:  $server_url"
    echo ""
}

# ============================================================
# 创建 systemd 服务
# ============================================================
create_systemd_service() {
    local svc_name="$1"
    local desc="$2"
    local exec_path="$3"
    local exec_args="$4"
    local work_dir="$5"

    step "创建 systemd 服务: ${svc_name}"

    cat > "/etc/systemd/system/${svc_name}.service" << EOF
[Unit]
Description=${desc}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${work_dir}
ExecStart=${exec_path} ${exec_args}
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${svc_name%.service}

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "$svc_name"
    info "已设置开机自启: ${svc_name}"
}

# ============================================================
# 启动服务
# ============================================================
start_service() {
    local svc_name="$1"

    # 如果已运行，先停止
    if systemctl is-active --quiet "$svc_name" 2>/dev/null; then
        warn "服务 ${svc_name} 已在运行，正在重启..."
        systemctl stop "$svc_name"
    fi

    step "启动服务: ${svc_name}"
    systemctl start "$svc_name"

    sleep 2
    if systemctl is-active --quiet "$svc_name"; then
        info "服务 ${svc_name} 启动成功 ✓"
    else
        warn "服务 ${svc_name} 可能未正常启动，请检查日志:"
        echo "  journalctl -u ${svc_name} -n 20"
    fi
}

# ============================================================
# 显示使用帮助
# ============================================================
show_tips() {
    echo ""
    echo -e "${BOLD}常用命令:${NC}"
    echo "  查看状态:   systemctl status hawkeye-server hawkeye-agent"
    echo "  查看日志:   journalctl -u hawkeye-server -f"
    echo "  停止服务:   systemctl stop hawkeye-server"
    echo "  重启服务:   systemctl restart hawkeye-agent"
    echo ""
    echo -e "${BOLD}卸载:${NC}"
    echo "  # 停用并移除服务"
    echo "  systemctl disable --now hawkeye-agent hawkeye-server"
    echo "  rm -f /etc/systemd/system/hawkeye-agent.service /etc/systemd/system/hawkeye-server.service"
    echo "  rm -rf ${INSTALL_DIR}"
    echo ""
    echo -e "${BOLD}安装 Agent 到其他机器:${NC}"
    echo "  curl -sSL https://raw.githubusercontent.com/${GITHUB_OWNER}/${GITHUB_REPO}/main/scripts/install.sh | bash -s -- --mode agent"
    echo ""
}

# ============================================================
# 主流程
# ============================================================

echo ""
echo -e "${BOLD}🦅  Hawkeye 安装脚本${NC}"
echo "  版本: ${VERSION}"
echo "  下载源: GitHub Releases (${GITHUB_OWNER}/${GITHUB_REPO})"
echo ""

detect_system
choose_mode

case "$MODE" in
    server)
        install_server
        ;;
    agent)
        install_agent ""
        ;;
    all)
        # 先装 Server，拿到本机 IP 后再装 Agent
        install_server

        # 获取本机 IP，自动填入 Agent 的 server-url
        local_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
        server_port="${PORT:-$SERVER_PORT}"
        auto_url="http://${local_ip}:${server_port}"

        install_agent "$auto_url"

        echo ""
        info "全套安装完成！"
        echo ""
        echo "  Server:     http://${local_ip}:${server_port}"
        echo "  Agent 端口: ${AGENT_PORT}"
        echo "  Agent 已自动注册到本机 Server"
        ;;
esac

show_tips
echo -e "${GREEN}安装完毕，祝使用愉快！ 🎉${NC}"
echo ""
