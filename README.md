# server-agent

Gửi metrics VPS về dpm-backend mỗi 5s.

## Chạy dev

```bash
cp .env.example .env   # sửa API_URL, SECRET, SERVER_NAME
go run .
```

## Build

```bash
# macOS
go build -o server-agent .

# Linux VPS (nhẹ hơn, stripped)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server-agent .
```

## Deploy lên VPS Linux

```bash
# 1. Build & copy
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o server-agent .
scp server-agent root@vps:/usr/local/bin/
rsync -avz --progress server-agent root@221.132.xx.xxx:~/server-agent/
rsync -avz --progress .env.production root@221.132.xx.xxx:~/server-agent/.env
rsync -avz --progress server-agent.service root@221.132.xx.xxx:~/server-agent/

# 2. Trên VPS: tạo config
mkdir -p /etc/server-agent /var/lib/server-agent
cat > /etc/server-agent/.env << 'EOF'
API_URL=https://be.dhn.io.vn/dpm/v1
SECRET=dhn_server_agent_2025
SERVER_NAME=web-01
# DATABASE_URL=postgres://user:pass@localhost:5432/postgres
EOF

# 3. Cài systemd service
chmod +x service-agent
cp server-agent.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now server-agent

# 4. Kiểm tra
systemctl status server-agent
journalctl -u server-agent -f
```

## Flag

```bash
server-agent -env /etc/server-agent/.env
```

## .env

```env
API_URL=http://localhost:7001/dpm/v1
SECRET=dhn_server_agent_2025
SERVER_NAME=web-01
# DATABASE_URL=postgres://user:pass@localhost:5432/postgres   # tuỳ chọn
```

`DATABASE_URL` chỉ cần nếu muốn theo dõi PostgreSQL. Nên tạo user riêng:

```sql
-- PG 14+
CREATE ROLE server_agent_monitor WITH LOGIN PASSWORD 'mật_khẩu_mạnh';
GRANT pg_read_all_stats TO server_agent_monitor;

-- PG 13 trở xuống
CREATE ROLE server_agent_monitor WITH LOGIN PASSWORD 'mật_khẩu_mạnh';
GRANT pg_monitor TO server_agent_monitor;
```

## Tài nguyên

| | Mức dùng |
|--|----------|
| RAM | ~25MB |
| CPU | < 0.05% |
| Binary | ~15MB |
# server-agent
