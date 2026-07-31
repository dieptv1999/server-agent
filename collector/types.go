package collector

type Metrics struct {
	IP            string          `json:"ip"`
	Hostname      string          `json:"hostname"`
	ServerName    string          `json:"server_name"`
	OS            string          `json:"os"`
	UptimeSeconds uint64          `json:"uptime_seconds"`
	CPUPercent    float64         `json:"cpu_percent"`
	CPUCores      int             `json:"cpu_cores"`
	CPUModel      string          `json:"cpu_model"`
	RAMUsedMB     uint64          `json:"ram_used_mb"`
	RAMTotalMB    uint64          `json:"ram_total_mb"`
	SwapUsedMB    uint64          `json:"swap_used_mb"`
	SwapTotalMB   uint64          `json:"swap_total_mb"`
	DiskUsedGB    float64         `json:"disk_used_gb"`
	DiskTotalGB   float64         `json:"disk_total_gb"`
	NetRXBytes    uint64          `json:"network_rx_bytes"`
	NetTXBytes    uint64          `json:"network_tx_bytes"`
	NetRXRate     uint64          `json:"network_rx_rate"`
	NetTXRate     uint64          `json:"network_tx_rate"`
	DiskReadOps   uint64          `json:"disk_read_ops"`
	DiskWriteOps  uint64          `json:"disk_write_ops"`
	Postgres      *PostgresStatus `json:"postgres"`
}
