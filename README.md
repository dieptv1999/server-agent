# server-agent

Gửi metrics VPS về dpm-backend mỗi 5s.

## Cài đặt (Linux)

```bash
curl -sSL https://raw.githubusercontent.com/dieptv1999/server-agent/main/install.sh | sudo bash
```

Script sẽ hỏi interactive: `API_URL`, `SECRET`, `SERVER_NAME` (tuỳ chọn), `DATABASE_URL` (tuỳ chọn). Tự động download binary + systemd service, enable & start.

Chạy lại lần 2 → cập nhật binary, reload service.

Tuỳ chọn thư mục cài:
```bash
curl ... | sudo bash -s -- -d /opt/server-agent
```

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
| Binary | ~7MB |
