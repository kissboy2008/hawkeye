#!/bin/bash
# Hawkeye Agent 控制脚本（Unraid，无 systemd）
# 用法: agent-ctl.sh {start|stop|restart|status|log|prepare}
# 说明: Unraid 的 /boot 是 FAT，文件没有可执行位，所以二进制常驻在 /boot/custom/hawkeye/，
#       启动时同步复制到内存盘 /usr/local/hawkeye/ 再执行（重启后自动重建）。
DIR=/boot/custom/hawkeye
SRC="$DIR/hawkeye-agent"
CFG="$DIR/agent.yaml"
RUN=/usr/local/hawkeye
BIN="$RUN/hawkeye-agent"
LOG=/var/log/hawkeye-agent.log
PIDFILE="$RUN/agent.pid"

prepare() {
    mkdir -p "$RUN"
    if [ ! -f "$BIN" ] || ! cmp -s "$SRC" "$BIN"; then
        cp -f "$SRC" "$BIN" || { echo "复制二进制失败: $SRC"; return 1; }
    fi
    chmod +x "$BIN"
    [ -f "$LOG" ] || : > "$LOG"
    return 0
}

is_running() {
    [ -f "$PIDFILE" ] || return 1
    local pid
    pid=$(cat "$PIDFILE" 2>/dev/null)
    [ -n "$pid" ] || return 1
    kill -0 "$pid" 2>/dev/null
}

start() {
    if is_running; then
        echo "hawkeye-agent 已在运行 (pid $(cat "$PIDFILE"))"
        return 0
    fi
    prepare || return 1
    nohup "$BIN" -config "$CFG" >> "$LOG" 2>&1 &
    echo $! > "$PIDFILE"
    sleep 1
    if is_running; then
        echo "hawkeye-agent 已启动 (pid $(cat "$PIDFILE"))"
    else
        echo "hawkeye-agent 启动失败，请查看 $LOG"
        return 1
    fi
}

stop() {
    local pid
    if [ -f "$PIDFILE" ]; then
        pid=$(cat "$PIDFILE" 2>/dev/null)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null
            sleep 1
            kill -9 "$pid" 2>/dev/null
            echo "hawkeye-agent 已停止 (pid $pid)"
        else
            echo "hawkeye-agent 未在运行"
        fi
        rm -f "$PIDFILE"
    else
        pkill -f "$RUN/hawkeye-agent -config" 2>/dev/null && echo "hawkeye-agent 已停止" || echo "hawkeye-agent 未在运行"
    fi
}

status() {
    if is_running; then
        echo "hawkeye-agent 运行中 (pid $(cat "$PIDFILE"))"
        "$BIN" -version 2>/dev/null || true
    else
        echo "hawkeye-agent 未运行"
        return 1
    fi
}

case "$1" in
    start)   start ;;
    stop)    stop ;;
    restart) stop; start ;;
    status)  status ;;
    prepare) prepare && echo "运行时文件已就绪: $BIN" ;;
    log)     tail -n 50 "$LOG" ;;
    *)       echo "用法: $0 {start|stop|restart|status|log|prepare}"; exit 2 ;;
esac
