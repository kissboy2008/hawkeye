#!/bin/bash
# ============================================================
#  Hawkeye Agent 一键安装（引导脚本）
#  从 GitHub Releases 下载完整安装脚本
#
#  用法:
#    curl -sSL https://raw.githubusercontent.com/kissboy2008/hawkeye/main/scripts/install.sh | bash -s -- --mode agent
#
#  或快速安装（带参数）:
#    bash <(curl -sSL ...) --mode agent --token YOUR_TOKEN --server-url http://1.2.3.4:18325
# ============================================================

set -euo pipefail

GITHUB_OWNER="kissboy2008"
GITHUB_REPO="hawkeye"
INSTALL_URL="https://raw.githubusercontent.com/${GITHUB_OWNER}/${GITHUB_REPO}/main/scripts/install.sh"

echo ">>> 正在获取安装脚本..."
exec bash <(curl -fsSL "$INSTALL_URL") --mode agent "$@"
