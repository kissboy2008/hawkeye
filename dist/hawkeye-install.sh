#!/bin/bash
#===============================================================================
#  Hawkeye 一键部署脚本
#  适用系统: Ubuntu / Debian / OpenWrt / Unraid
#  版本: 2.0.0
#  用法: curl -fsSL https://www.icloud325.cn/sh/hawkeye-install.sh | bash
#===============================================================================
set -euo pipefail

#============================ 版本信息 ============================
VERSION="${HAWKEYE_VERSION:-1.5.1}"
ALIYUN_BASE="https://www.icloud325.cn/sh"
GITHUB_BASE="https://raw.githubusercontent.com/kissboy2008/hawkeye/master/dist"

#============================ 全局状态 ============================
OS_TYPE=""          # ubuntu / debian / openwrt / unraid
CPU_ARCH=""         # amd64 / arm64
DOWNLOAD_BASE=""    # 根据 GEO 选择
INSTALL_MODE=""     # full / server / agent
SERVER_PORT="18325"
AGENT_SERVER_URL="" # Agent 上报地址
TOKEN=""            # 认证 Token
SERVER_INSTALL_DIR=""
AGENT_INSTALL_DIR=""

#============================ 输出函数 ============================
green()  { echo -e "\033[0;32m$*\033[0m"; }
yellow() { echo -e "\033[1;33m$*\033[0m"; }
red()    { echo -e "\033[0;31m$*\033[0m"; }
cyan()   { echo -e "\033[0;36m$*\033[0m"; }
bold()   { echo -e "\033[1m$*\033[0m"; }

ok()    { green "  [✓] $*"; }
info()  { cyan  "  [i] $*"; }
warn()  { yellow "  [!] $*"; }
err()   { red   "  [✗] $*"; exit 1; }

#============================ 工具函数 ============================
generate_token() {
    if command -v uuidgen &>/dev/null; then
        uuidgen | tr -d '[:space:]'
    elif command -v hexdump &>/dev/null; then
        od -A n -t x -N 16 /dev/urandom | tr -d '[:space:]'
    else
        head -c 16 /dev/urandom | md5sum | cut -d' ' -f1
    fi
}

have_cmd() { command -v "$1" &>/dev/null; }

download_file() {
    local url="$1"
    local dest="$2"
    local min_size="${3:-2000000}"

    info "下载: ${url}"
    if have_cmd curl; then
        curl -fSL --progress-bar "$url" -o "/tmp/hawkeye_dl.tmp" 2>/dev/null || {
            warn "curl 下载失败: ${url}"
            return 1
        }
    elif have_cmd wget; then
        wget -q --show-progress "$url" -O "/tmp/hawkeye_dl.tmp" 2>/dev/null || {
            warn "wget 下载失败: ${url}"
            return 1
        }
    else
        err "请先安装 curl 或 wget"
    fi

    local size
    size=$(stat -c%s "/tmp/hawkeye_dl.tmp" 2>/dev/null || stat -f%z "/tmp/hawkeye_dl.tmp" 2>/dev/null || echo 0)
    if [[ "$size" -lt "$min_size" ]]; then
        rm -f "/tmp/hawkeye_dl.tmp"
        warn "下载文件异常偏小 (${size} bytes)"
        return 1
    fi

    mv "/tmp/hawkeye_dl.tmp" "$dest"
    chmod +x "$dest"
    ok "下载完成 (${size} bytes)"
}

#============================ 环境检测 ============================
detect_os() {
    if [ -f /etc/unraid-version ] || grep -q 'UNRAID' /proc/version 2>/dev/null; then
        OS_TYPE="unraid"
    elif [ -f /etc/openwrt_release ] || grep -qi 'openwrt' /etc/os-release 2>/dev/null; then
        OS_TYPE="openwrt"
    elif grep -qi 'ubuntu' /etc/os-release 2>/dev/null; then
        OS_TYPE="ubuntu"
    elif grep -qi 'debian' /etc/os-release 2>/dev/null; then
        OS_TYPE="debian"
    else
        warn "无法识别系统类型，尝试按 Debian 处理"
        OS_TYPE="debian"
    fi

    case $(uname -m) in
        x86_64|amd64) CPU_ARCH="amd64" ;;
        aarch64|arm64) CPU_ARCH="arm64" ;;
        *) err "不支持的 CPU 架构: $(uname -m)" ;;
    esac

    ok "系统: ${OS_TYPE} / ${CPU_ARCH}"
}

detect_geo() {
    if ping -c1 -W2 google.com &>/dev/null; then
        DOWNLOAD_BASE="$GITHUB_BASE"
        ok "下载源: GitHub"
    else
        DOWNLOAD_BASE="$ALIYUN_BASE"
        ok "下载源: 阿里云"
    fi
}

#============================ 目录与路径 ============================
setup_paths() {
    if [ "$OS_TYPE" = "unraid" ]; then
        SERVER_INSTALL_DIR="/boot/custom/hawkeye"
        AGENT_INSTALL_DIR="/boot/custom/hawkeye"
    else
        SERVER_INSTALL_DIR="/opt/hawkeye"
        AGENT_INSTALL_DIR="/opt/hawkeye"
    fi
}

#============================ 下载二进制 ============================
download_server() {
    local dest="${SERVER_INSTALL_DIR}/hawkeye-server"
    local url="${DOWNLOAD_BASE}/hawkeye-server"
    local fallback="${GITHUB_BASE}/hawkeye-server"

    mkdir -p "$SERVER_INSTALL_DIR"
    download_file "$url" "$dest" || download_file "$fallback" "$dest"
}

download_agent() {
    local fname="hawkeye-agent-${CPU_ARCH}"
    local dest="${AGENT_INSTALL_DIR}/hawkeye-agent"
    local url="${DOWNLOAD_BASE}/${fname}"
    local fallback="${GITHUB_BASE}/${fname}"

    mkdir -p "$AGENT_INSTALL_DIR"
    download_file "$url" "$dest" || download_file "$fallback" "$dest"
}

#============================ 生成配置 ============================
generate_server_config() {
    local cfg="${SERVER_INSTALL_DIR}/server.yaml"
    mkdir -p "${SERVER_INSTALL_DIR}/data" "${SERVER_INSTALL_DIR}/logs"

    cat > "$cfg" << YAML
# Hawkeye Server 配置
# 生成于: $(date '+%Y-%m-%d %H:%M:%S')

server:
  listen: ":${SERVER_PORT}"
  mode: "release"

database:
  path: "${SERVER_INSTALL_DIR}/data/monitoring.db"
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
  token: "${TOKEN}"

backup:
  enabled: true
  dir: "${SERVER_INSTALL_DIR}/data/backups"
  max_keep: 7
  interval_h: 24
YAML
    ok "生成 server.yaml"
}

generate_agent_config() {
    local cfg="${AGENT_INSTALL_DIR}/agent.yaml"

    cat > "$cfg" << YAML
# Hawkeye Agent 配置
# 生成于: $(date '+%Y-%m-%d %H:%M:%S')

server:
  listen: ":32518"
  url: "${AGENT_SERVER_URL}"
  push_interval_s: 30

auth:
  token: "${TOKEN}"

collect:
  network_interfaces: []
YAML
    ok "生成 agent.yaml"
}

#============================ systemd (Ubuntu/Debian) ============================
install_systemd_service() {
    local name="$1"
    local desc="$2"
    local bin="$3"
    local args="$4"
    local wd="$5"

    cat > "/etc/systemd/system/${name}.service" << UNIT
[Unit]
Description=${desc}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${wd}
ExecStart=${bin} ${args}
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${name}

[Install]
WantedBy=multi-user.target
UNIT

    systemctl daemon-reload 2>/dev/null || true
    systemctl enable "$name" 2>/dev/null || true
    ok "已创建 ${name}.service (systemd)"
}

#============================ procd (OpenWrt) ============================
install_openwrt_service() {
    local name="$1"
    local bin="$2"
    local args="$3"
    local wd="$4"

    local init_script="/etc/init.d/${name}"

    cat > "$init_script" << 'INIT'
#!/bin/sh /etc/rc.common
START=95
USE_PROCD=1

NAME="__NAME__"
BIN="__BIN__"
ARGS="__ARGS__"
WD="__WD__"

start_service() {
    procd_open_instance
    procd_set_param command "$BIN" $ARGS
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}

stop_service() {
    killall "$(basename "$BIN")" 2>/dev/null
}

status_service() {
    if pgrep -f "$(basename "$BIN")" > /dev/null; then
        echo "running"
    else
        echo "stopped"
    fi
}
INIT

    sed -i "s|__NAME__|${name}|g" "$init_script"
    sed -i "s|__BIN__|${bin}|g" "$init_script"
    sed -i "s|__ARGS__|${args}|g" "$init_script"
    sed -i "s|__WD__|${wd}|g" "$init_script"

    chmod +x "$init_script"
    "$init_script" enable &>/dev/null || true
    ok "已创建 ${name} (procd)"
}

#============================ Unraid 自启 ============================
install_unraid_agent() {
    local dir="$AGENT_INSTALL_DIR"
    mkdir -p "${dir}/logs" "${dir}/scripts"

    # 生成 agent-ctl.sh
    cat > "${dir}/scripts/agent-ctl.sh" << 'CTL'
#!/bin/bash
AGENT_DIR="/boot/custom/hawkeye"
AGENT_BIN="$AGENT_DIR/hawkeye-agent"
AGENT_CONFIG="$AGENT_DIR/agent.yaml"
AGENT_LOG="$AGENT_DIR/logs/agent.log"
AGENT_PID="/var/run/hawkeye-agent.pid"

is_running() { [[ -f $AGENT_PID ]] && kill -0 $(cat $AGENT_PID) 2>/dev/null; }

case "${1:-}" in
  start)
    if is_running; then echo "Already running (PID: $(cat $AGENT_PID))"; exit 0; fi
    echo "Starting hawkeye-agent..."
    nohup $AGENT_BIN -config $AGENT_CONFIG >> $AGENT_LOG 2>&1 &
    echo $! > $AGENT_PID
    sleep 1
    is_running && echo "Started (PID: $(cat $AGENT_PID))" || echo "Failed!"
    ;;
  stop)
    is_running || { echo "Not running"; exit 0; }
    PID=$(cat $AGENT_PID)
    kill $PID 2>/dev/null
    sleep 1
    rm -f $AGENT_PID
    echo "Stopped"
    ;;
  restart) $0 stop; sleep 1; $0 start ;;
  status)
    if is_running; then echo "Running (PID: $(cat $AGENT_PID))"; else echo "Not running"; fi
    ;;
  log) tail -f $AGENT_LOG ;;
  *) echo "Usage: $0 {start|stop|restart|status|log}"; exit 1 ;;
esac
CTL
    chmod +x "${dir}/scripts/agent-ctl.sh"

    # 写入 /boot/config/go（追加自启行，避免重复）
    local go_file="/boot/config/go"
    local start_line="${dir}/scripts/agent-ctl.sh start"

    if [ -f "$go_file" ]; then
        if ! grep -qF "$start_line" "$go_file" 2>/dev/null; then
            echo "$start_line" >> "$go_file"
            ok "已添加自启到 /boot/config/go"
        else
            info "/boot/config/go 中已存在自启条目，跳过"
        fi
    else
        warn "/boot/config/go 不存在，无法添加自启"
    fi

    ok "Unraid Agent 安装完成"
}

#============================ 服务安装路由 ============================
install_service() {
    local mode="$1"  # server / agent

    if [ "$mode" = "server" ]; then
        local name="hawkeye-server"
        local bin="${SERVER_INSTALL_DIR}/hawkeye-server"
        local args="-config ${SERVER_INSTALL_DIR}/server.yaml"
        local wd="$SERVER_INSTALL_DIR"
        local desc="Hawkeye Server - 管理面板"
    else
        local name="hawkeye-agent"
        local bin="${AGENT_INSTALL_DIR}/hawkeye-agent"
        local args="-config ${AGENT_INSTALL_DIR}/agent.yaml"
        local wd="$AGENT_INSTALL_DIR"
        local desc="Hawkeye Agent - 监控采集"
    fi

    case "$OS_TYPE" in
        ubuntu|debian)
            install_systemd_service "$name" "$desc" "$bin" "$args" "$wd"
            ;;
        openwrt)
            install_openwrt_service "$name" "$bin" "$args" "$wd"
            ;;
        unraid)
            # Unraid 只装 Agent，Server 不走这里
            return 0
            ;;
    esac
}

start_service() {
    local name="$1"

    case "$OS_TYPE" in
        ubuntu|debian)
            systemctl restart "$name" &>/dev/null || true
            sleep 1
            if systemctl is-active --quiet "$name" 2>/dev/null; then
                ok "${name} 已启动"
            else
                warn "${name} 启动失败，请检查: journalctl -u ${name} -n 20"
            fi
            ;;
        openwrt)
            /etc/init.d/"$name" restart &>/dev/null
            sleep 1
            if /etc/init.d/"$name" status 2>/dev/null | grep -q 'running'; then
                ok "${name} 已启动"
            else
                warn "${name} 启动失败，请检查日志"
            fi
            ;;
        unraid)
            "${AGENT_INSTALL_DIR}/scripts/agent-ctl.sh" start
            ;;
    esac
}

#============================ 安装 Server ============================
install_server() {
    echo ""
    bold "--- 安装 Hawkeye Server ---"

    read -p "  监听端口 (默认 ${SERVER_PORT}): " input_port
    SERVER_PORT="${input_port:-$SERVER_PORT}"

    if [ "$OS_TYPE" = "unraid" ]; then
        warn "Unraid 不建议安装 Server，请在其他机器部署"
        return 1
    fi

    download_server
    generate_server_config
    install_service "server"
    start_service "hawkeye-server"
}

#============================ 安装 Agent ============================
install_agent() {
    echo ""
    bold "--- 安装 Hawkeye Agent ---"

    # 全套模式已自动设置，跳过提示
    if [ -z "$AGENT_SERVER_URL" ]; then
        read -p "  Server 地址 (例 http://10.0.0.3:${SERVER_PORT}): " AGENT_SERVER_URL
        if [ -z "$AGENT_SERVER_URL" ]; then
            err "Server 地址不能为空"
        fi
    else
        info "Server 地址: ${AGENT_SERVER_URL}"
    fi

    # 独立安装 Agent 时生成 Token，全套时复用已生成的 Token
    if [ -z "$TOKEN" ]; then
        TOKEN=$(generate_token)
        info "自动生成 Token: ${TOKEN}"
    fi

    download_agent
    generate_agent_config

    if [ "$OS_TYPE" = "unraid" ]; then
        install_unraid_agent
    else
        install_service "agent"
    fi

    start_service "hawkeye-agent"
}

#============================ 卸载 ============================
uninstall_hawkeye() {
    echo ""
    bold "--- Hawkeye 卸载 ---"
    echo ""

    # ---- 检测已安装组件 ----
    local found_server=false
    local found_agent=false
    local found_dirs=()

    # Ubuntu/Debian systemd
    if [ -f /etc/systemd/system/hawkeye-server.service ]; then
        found_server=true
    fi
    if [ -f /etc/systemd/system/hawkeye-agent.service ]; then
        found_agent=true
    fi

    # OpenWrt procd
    if [ -f /etc/init.d/hawkeye-server ]; then
        found_server=true
    fi
    if [ -f /etc/init.d/hawkeye-agent ]; then
        found_agent=true
    fi

    # 安装目录
    [ -d /opt/hawkeye ] && found_dirs+=("/opt/hawkeye")
    [ -d /boot/custom/hawkeye ] && found_dirs+=("/boot/custom/hawkeye")

    # Unraid go 文件条目
    if [ -f /boot/config/go ]; then
        if grep -qF 'hawkeye' /boot/config/go 2>/dev/null; then
            found_agent=true
        fi
    fi

    # ---- 没有检测到任何东西 ----
    if ! $found_server && ! $found_agent && [ ${#found_dirs[@]} -eq 0 ]; then
        info "未检测到 Hawkeye 安装，无需卸载"
        return 0
    fi

    # ---- 显示检测结果 ----
    echo "检测到以下 Hawkeye 组件:"
    $found_server && yellow "  • Hawkeye Server"
    $found_agent  && yellow "  • Hawkeye Agent"
    for d in "${found_dirs[@]}"; do
        yellow "  • 目录: ${d}"
    done
    echo ""

    # ---- 确认 ----
    read -p "  确认卸载? 这将删除以上所有内容 [y/N]: " confirm
    if [ "${confirm,,}" != "y" ] && [ "${confirm,,}" != "yes" ]; then
        info "已取消卸载"
        return 0
    fi

    echo ""
    info "开始卸载..."

    # ---- 停止服务 ----
    for svc in hawkeye-server hawkeye-agent; do
        if [ -f /etc/systemd/system/${svc}.service ]; then
            systemctl stop "$svc" 2>/dev/null || true
            ok "已停止 ${svc}"
        fi
        if [ -f "/etc/init.d/${svc}" ]; then
            "/etc/init.d/${svc}" stop 2>/dev/null || true
            ok "已停止 ${svc}"
        fi
    done

    # Unraid: 通过 ctl 脚本停止
    if [ -f /boot/custom/hawkeye/scripts/agent-ctl.sh ]; then
        /boot/custom/hawkeye/scripts/agent-ctl.sh stop 2>/dev/null || true
        ok "已停止 hawkeye-agent (Unraid)"
    fi

    # ---- 删除服务文件 ----
    for svc in hawkeye-server hawkeye-agent; do
        if [ -f "/etc/systemd/system/${svc}.service" ]; then
            systemctl disable "$svc" 2>/dev/null || true
            rm -f "/etc/systemd/system/${svc}.service"
            ok "已删除 ${svc}.service"
        fi
        if [ -f "/etc/init.d/${svc}" ]; then
            "/etc/init.d/${svc}" disable 2>/dev/null || true
            rm -f "/etc/init.d/${svc}"
            ok "已删除 /etc/init.d/${svc}"
        fi
    done

    systemctl daemon-reload 2>/dev/null || true

    # ---- 清理 Unraid go 文件 ----
    if [ -f /boot/config/go ]; then
        if grep -qF 'hawkeye' /boot/config/go 2>/dev/null; then
            sed -i '/hawkeye/d' /boot/config/go
            ok "已清理 /boot/config/go 中 hawkeye 条目"
        fi
    fi

    # ---- 删除目录 ----
    for d in "${found_dirs[@]}"; do
        rm -rf "$d"
        ok "已删除目录: ${d}"
    done

    echo ""
    green "Hawkeye 已完全卸载，来去无痕 ✨"
    echo ""
}

#============================ 汇总输出 ============================
show_summary() {
    echo ""
    echo "═══════════════════════════════════════════"
    green "  Hawkeye 安装完成 ✓"
    echo "───────────────────────────────────────────"

    case "$INSTALL_MODE" in
        full|server)
            echo "  Server 地址:    http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'localhost'):${SERVER_PORT}"
            echo "  配置目录:       ${SERVER_INSTALL_DIR}"
            echo "  数据目录:       ${SERVER_INSTALL_DIR}/data"
            ;;
    esac

    case "$INSTALL_MODE" in
        full|agent)
            echo "  Agent Token:    ${TOKEN}"
            echo "  上报地址:       ${AGENT_SERVER_URL}"
            ;;
    esac

    echo "───────────────────────────────────────────"
    if [ "$INSTALL_MODE" = "agent" ] && [ "$OS_TYPE" != "unraid" ]; then
        yellow "  ⚠ 请将此 Token 添加到 Server 配置的 auth.token 字段"
    fi

    # 服务状态
    echo ""
    case "$OS_TYPE" in
        ubuntu|debian)
            systemctl is-active hawkeye-server &>/dev/null && ok "hawkeye-server: running" || true
            systemctl is-active hawkeye-agent &>/dev/null && ok "hawkeye-agent: running" || true
            ;;
        openwrt)
            /etc/init.d/hawkeye-server status 2>/dev/null | grep -q running && ok "hawkeye-server: running" || true
            /etc/init.d/hawkeye-agent status 2>/dev/null | grep -q running && ok "hawkeye-agent: running" || true
            ;;
        unraid)
            "${AGENT_INSTALL_DIR}/scripts/agent-ctl.sh" status 2>/dev/null
            ;;
    esac

    echo "═══════════════════════════════════════════"
    echo ""
}

#============================ 主入口 ============================
main() {
    echo ""
    bold "Hawkeye 一键部署脚本 v${VERSION}"
    echo ""

    # 如果是管道执行 (curl | bash)，重新连接终端以支持交互输入
    if [ ! -t 0 ]; then
        exec < /dev/tty
    fi

    # 需要 root
    if [ "$(id -u)" -ne 0 ]; then
        err "请使用 root 运行: curl ... | sudo bash"
    fi

    detect_os
    detect_geo
    setup_paths

    echo ""
    echo "请选择操作:"
    echo "  1) 安装 Hawkeye"
    echo "  2) 仅安装 Server"
    echo "  3) 仅安装 Agent"
    echo "  4) 卸载 Hawkeye"
    read -p "  请输入 [1-4]: " choice
    echo ""

    case "$choice" in
        1) INSTALL_MODE="full" ;;
        2) INSTALL_MODE="server" ;;
        3) INSTALL_MODE="agent" ;;
        4) uninstall_hawkeye; exit 0 ;;
        *) err "无效选择，请输入 1-4" ;;
    esac

    # 全套模式：先生成统一 Token
    if [ "$INSTALL_MODE" = "full" ]; then
        TOKEN=$(generate_token)
        info "自动生成统一 Token: ${TOKEN}"
    fi

    case "$INSTALL_MODE" in
        full)
            install_server
            AGENT_SERVER_URL="http://127.0.0.1:${SERVER_PORT}"
            install_agent
            ;;
        server)
            install_server
            ;;
        agent)
            install_agent
            ;;
    esac

    show_summary
}

main
