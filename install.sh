#!/bin/bash
set -e

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

GITHUB_USER="dieptv1999"
GITHUB_REPO="server-agent"
GITHUB_BRANCH="main"
RAW_BASE="https://raw.githubusercontent.com/${GITHUB_USER}/${GITHUB_REPO}/${GITHUB_BRANCH}"

DEFAULT_INSTALL_DIR="/root/server-agent"
INSTALL_DIR="$DEFAULT_INSTALL_DIR"

usage() {
  echo "Usage: curl ... | bash [-d <install_dir>]"
  echo "  -d  Thư mục cài đặt (default: $DEFAULT_INSTALL_DIR)"
  exit 1
}

while getopts "d:h" opt; do
  case $opt in
    d) INSTALL_DIR="$OPTARG" ;;
    h) usage ;;
    *) usage ;;
  esac
done

ENV_FILE="${INSTALL_DIR}/.env"
BIN_PATH="${INSTALL_DIR}/server-agent"
SVC_PATH="/etc/systemd/system/server-agent.service"

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}   Server Agent - Cài đặt tự động${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo -e "${BOLD}Thư mục cài đặt:${NC} ${INSTALL_DIR}"
echo ""

# ---- root check ----
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}Vui lòng chạy với sudo hoặc root.${NC}"
    exit 1
fi

# ---- load old .env as defaults ----
declare -A OLD_ENV
if [ -f "$ENV_FILE" ]; then
    echo -e "${YELLOW}Đã phát hiện .env cũ, dùng làm giá trị mặc định...${NC}"
    while IFS='=' read -r key value; do
        key=$(echo "$key" | xargs)
        value=$(echo "$value" | xargs)
        [ -z "$key" ] && continue
        [[ "$key" =~ ^# ]] && continue
        OLD_ENV["$key"]="$value"
    done < "$ENV_FILE"
    echo ""
fi

# ---- prompts ----
prompt() {
    local var=$1 prompt_text=$2 required=$3
    local default="${OLD_ENV[$var]:-}"
    local val

    while true; do
        if [ -n "$default" ]; then
            read -r -p "$(echo -e "${BOLD}${prompt_text}${NC} [${default}]: ")" val
            val="${val:-$default}"
        else
            read -r -p "$(echo -e "${BOLD}${prompt_text}${NC}: ")" val
        fi

        if [ "$required" = "yes" ] && [ -z "$val" ]; then
            echo -e "${RED}  Giá trị này là bắt buộc.${NC}"
            continue
        fi
        break
    done
    echo "$val"
}

API_URL=$(prompt "API_URL" "API URL (vd: https://be.dhn.io.vn/dpm/v1)" "yes")
SECRET=$(prompt "SECRET" "Secret key (phải khớp SERVER_AGENT_SECRET ở backend)" "yes")
SERVER_NAME=$(prompt "SERVER_NAME" "Tên server hiển thị (bỏ trống = hostname)" "no")
DATABASE_URL=$(prompt "DATABASE_URL" "PostgreSQL DSN (bỏ trống nếu không theo dõi PG)" "no")

# ---- create dirs ----
mkdir -p "$INSTALL_DIR"

# ---- write .env ----
cat > "$ENV_FILE" << EOF
API_URL=${API_URL}
SECRET=${SECRET}
SERVER_NAME=${SERVER_NAME}
DATABASE_URL=${DATABASE_URL}
EOF
echo -e "${GREEN}.env đã được ghi → ${ENV_FILE}${NC}"

# ---- download binary ----
echo ""
echo -e "${CYAN}Đang tải binary...${NC}"
curl -sSL -o "$BIN_PATH" "${RAW_BASE}/server-agent"
chmod +x "$BIN_PATH"
echo -e "${GREEN}Binary đã tải → ${BIN_PATH}${NC}"

# ---- download & install service ----
echo ""
echo -e "${CYAN}Đang tải systemd service...${NC}"
curl -sSL -o /tmp/server-agent.service "${RAW_BASE}/server-agent.service"
sed -i "s|__INSTALL_DIR__|${INSTALL_DIR}|g" /tmp/server-agent.service
cp /tmp/server-agent.service "$SVC_PATH"
rm -f /tmp/server-agent.service
echo -e "${GREEN}Service đã cài → ${SVC_PATH}${NC}"

# ---- systemctl ----
echo ""
systemctl daemon-reload
systemctl enable server-agent

if systemctl is-active --quiet server-agent; then
    echo -e "${YELLOW}Đang khởi động lại service...${NC}"
    systemctl restart server-agent
else
    echo -e "${GREEN}Đang khởi động service...${NC}"
    systemctl start server-agent
fi

sleep 1

echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${GREEN}${BOLD}  Cài đặt hoàn tất!${NC}"
echo ""
systemctl status server-agent --no-pager -l || true
echo ""
echo -e "${CYAN}Logs:${NC} journalctl -u server-agent -f"
echo -e "${CYAN}Stop:${NC} systemctl stop server-agent"
echo -e "${CYAN}Start:${NC} systemctl start server-agent"
echo -e "${CYAN}Restart:${NC} systemctl restart server-agent"
echo -e "${CYAN}========================================${NC}"
