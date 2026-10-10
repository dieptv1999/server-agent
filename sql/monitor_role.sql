-- Tạo role monitor riêng cho server-agent (không dùng superuser).
-- Role này chỉ đọc được stats/catalog, KHÔNG đọc được dữ liệu user table.

-- PostgreSQL 14+ : pg_read_all_stats đủ cho cả pg_stat_replication và pg_replication_slots.
CREATE ROLE server_agent_monitor WITH LOGIN PASSWORD 'THAY_BANG_MAT_KHAU_MANH';
GRANT pg_read_all_stats TO server_agent_monitor;

-- PostgreSQL 10-13 :
--   pg_monitor đủ cho pg_stat_replication, nhưng pg_replication_slots vẫn cần superuser.
--   Khuyến nghị nâng cấp lên PG14+ thay vì cấp superuser cho agent.
-- CREATE ROLE server_agent_monitor WITH LOGIN PASSWORD 'THAY_BANG_MAT_KHAU_MANH';
-- GRANT pg_monitor TO server_agent_monitor;
