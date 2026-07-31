package collector

import (
	"net"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
)

var prevNetIO []psnet.IOCountersStat
var prevDiskIO map[string]disk.IOCountersStat

func collectNetworkRates() (rxRate, txRate uint64) {
	io, err := psnet.IOCounters(false)
	if err != nil || len(io) == 0 {
		return 0, 0
	}
	current := io[0]
	if prevNetIO != nil && len(prevNetIO) > 0 {
		rxRate = current.BytesRecv - prevNetIO[0].BytesRecv
		txRate = current.BytesSent - prevNetIO[0].BytesSent
	}
	prevNetIO = io
	return
}

func collectDiskIORates() (readOps, writeOps uint64) {
	io, err := disk.IOCounters()
	if err != nil {
		return 0, 0
	}
	if prevDiskIO == nil {
		prevDiskIO = io
		return 0, 0
	}
	for name, current := range io {
		prev, ok := prevDiskIO[name]
		if !ok {
			prevDiskIO[name] = current
			continue
		}
		readOps += current.ReadCount - prev.ReadCount
		writeOps += current.WriteCount - prev.WriteCount
		prevDiskIO[name] = current
	}
	return
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "unknown"
	}
	defer conn.Close()
	ip, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return "unknown"
	}
	return ip
}

func CollectAll() *Metrics {
	hostname, _ := os.Hostname()

	info, _ := host.Info()
	osStr := info.Platform + " " + info.PlatformVersion

	uptime, _ := host.Uptime()

	cpuPercent, _ := cpu.Percent(0, false)
	var cpuPct float64
	if len(cpuPercent) > 0 {
		cpuPct = cpuPercent[0]
	}

	cpuInfo, _ := cpu.Info()
	cpuModel := ""
	cores := runtime.NumCPU()
	if len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
		cores = int(cpuInfo[0].Cores)
	}

	vmem, _ := mem.VirtualMemory()
	var ramUsed, ramTotal uint64
	if vmem != nil {
		ramUsed = vmem.Used >> 20
		ramTotal = vmem.Total >> 20
	}

	smap, _ := mem.SwapMemory()
	var swapUsed, swapTotal uint64
	if smap != nil {
		swapUsed = smap.Used >> 20
		swapTotal = smap.Total >> 20
	}

	diskUsage, _ := disk.Usage("/")
	var diskUsedGB, diskTotalGB float64
	if diskUsage != nil {
		diskUsedGB = float64(diskUsage.Used) / 1e9
		diskTotalGB = float64(diskUsage.Total) / 1e9
	}

	netIO, _ := psnet.IOCounters(false)
	var netRX, netTX uint64
	if len(netIO) > 0 {
		netRX = netIO[0].BytesRecv
		netTX = netIO[0].BytesSent
	}

	rxRate, txRate := collectNetworkRates()
	readOps, writeOps := collectDiskIORates()

	ip := getLocalIP()

	return &Metrics{
		IP:            ip,
		Hostname:      hostname,
		OS:            osStr,
		UptimeSeconds: uptime,
		CPUPercent:    cpuPct,
		CPUCores:      cores,
		CPUModel:      cpuModel,
		RAMUsedMB:     ramUsed,
		RAMTotalMB:    ramTotal,
		SwapUsedMB:    swapUsed,
		SwapTotalMB:   swapTotal,
		DiskUsedGB:    diskUsedGB,
		DiskTotalGB:   diskTotalGB,
		NetRXBytes:    netRX,
		NetTXBytes:    netTX,
		NetRXRate:     rxRate,
		NetTXRate:     txRate,
		DiskReadOps:   readOps,
		DiskWriteOps:  writeOps,
	}
}
