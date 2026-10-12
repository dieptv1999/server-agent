package collector

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const availHours = 12

type PeerLink struct {
	IP             string   `json:"ip"`
	Name           string   `json:"name"`
	Online         bool     `json:"online"`
	LastSeen       int64    `json:"last_seen"`
	DownSince      int64    `json:"down_since"`
	Flapping       bool     `json:"flapping"`
	Availability   []uint32 `json:"availability"`
	AvailHourStart int64    `json:"avail_hour_start"`
}

type LinkReport struct {
	IP         string     `json:"ip"`
	Hostname   string     `json:"hostname"`
	ServerName string     `json:"server_name"`
	Links      []PeerLink `json:"links"`
}

type PeerConfig struct {
	Name string
	IP   string
}

type peerState struct {
	name           string
	online         bool
	reportedOnline bool
	lastSeen       time.Time
	downSince      time.Time
	downConfirmed  bool
	flapping       bool
	flips          []time.Time
	buckets        map[int64]uint32
}

type LinkMonitor struct {
	peers         []PeerConfig
	port          int
	intervalMs    int
	timeoutMs     int
	keepaliveS    int
	downConfirmMs int

	ownIP    string
	ownName  string
	hostname string

	conn *net.UDPConn

	mu         sync.Mutex
	state      map[string]*peerState
	seenSource map[string]bool
	out        chan LinkReport
	lastFlush  time.Time
	stop       chan struct{}
}

func NewLinkMonitor(peersRaw, ownName, hostname string) *LinkMonitor {
	peers := ParsePeers(peersRaw)
	m := &LinkMonitor{
		peers:         peers,
		port:          envInt("HEARTBEAT_PORT", 9999),
		intervalMs:    envInt("HEARTBEAT_INTERVAL_MS", 1000),
		timeoutMs:     envInt("HEARTBEAT_TIMEOUT_MS", 3000),
		keepaliveS:    envInt("HEARTBEAT_KEEPALIVE_S", 60),
		downConfirmMs: envInt("HEARTBEAT_DOWN_CONFIRM_MS", 5000),
		ownName:       ownName,
		hostname:      hostname,
		state:         make(map[string]*peerState),
		seenSource:    make(map[string]bool),
		out:           make(chan LinkReport, 16),
		stop:          make(chan struct{}),
	}
	m.ownIP = os.Getenv("HEARTBEAT_SELF_IP")
	if m.ownIP == "" {
		m.ownIP = getLocalIP()
	}
	for _, pc := range peers {
		m.state[pc.IP] = &peerState{name: pc.Name, buckets: make(map[int64]uint32)}
	}
	return m
}

func ParsePeers(s string) []PeerConfig {
	var peers []PeerConfig
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := ""
		ip := part
		if i := strings.LastIndex(part, ":"); i > 0 {
			left := part[:i]
			right := part[i+1:]
			if net.ParseIP(left) == nil && net.ParseIP(right) != nil {
				name = left
				ip = right
			}
		}
		peers = append(peers, PeerConfig{Name: name, IP: ip})
	}
	return peers
}

func (m *LinkMonitor) Start() error {
	if len(m.peers) == 0 {
		return fmt.Errorf("no peers configured")
	}
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort("0.0.0.0", strconv.Itoa(m.port)))
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	m.conn = conn
	go m.recvLoop()
	go m.runLoop()
	return nil
}

func (m *LinkMonitor) Stop() {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	if m.conn != nil {
		m.conn.Close()
	}
}

func (m *LinkMonitor) Out() <-chan LinkReport { return m.out }

func (m *LinkMonitor) recvLoop() {
	buf := make([]byte, 512)
	for {
		n, addr, err := m.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-m.stop:
				return
			default:
				log.Printf("link recv error: %v", err)
				continue
			}
		}
		var hb struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(buf[:n], &hb); err != nil {
			continue
		}
		peerIP := addr.IP.String()
		now := time.Now()
		m.mu.Lock()
		st := m.state[peerIP]
		if st == nil {
			if !m.seenSource[peerIP] {
				m.seenSource[peerIP] = true
				log.Printf("link: nhận gói UDP từ %s (name=%q) NHƯNG không khớp peer nào trong MONITOR_PEERS -> bỏ. Kiểm tra lại IP nguồn thật / cấu hình.", peerIP, hb.Name)
			}
			m.mu.Unlock()
			continue
		}
		if hb.Name != "" {
			st.name = hb.Name
		}
		if st.online {
			st.lastSeen = now
		} else {
			m.closeDown(st, now)
			st.online = true
			st.lastSeen = now
			st.downConfirmed = false
			st.registerFlip(now)
			log.Printf("link: peer %s (%s) UP", peerIP, st.name)
		}
		m.mu.Unlock()
	}
}

func (m *LinkMonitor) runLoop() {
	sendTick := time.NewTicker(time.Duration(m.intervalMs) * time.Millisecond)
	keepaliveTick := time.NewTicker(time.Duration(m.keepaliveS) * time.Second)
	defer sendTick.Stop()
	defer keepaliveTick.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-sendTick.C:
			m.sendHeartbeats()
			m.check()
			m.flush(false)
		case <-keepaliveTick.C:
			m.check()
			m.flush(true)
		}
	}
}

func (m *LinkMonitor) sendHeartbeats() {
	payload, err := json.Marshal(map[string]string{"name": m.ownName, "ip": m.ownIP})
	if err != nil {
		return
	}
	for _, pc := range m.peers {
		addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(pc.IP, strconv.Itoa(m.port)))
		if err != nil {
			continue
		}
		m.conn.WriteToUDP(payload, addr)
	}
}

func (m *LinkMonitor) check() {
	now := time.Now()
	timeout := time.Duration(m.timeoutMs) * time.Millisecond
	confirm := time.Duration(m.downConfirmMs) * time.Millisecond
	m.mu.Lock()
	defer m.mu.Unlock()
	for ip, st := range m.state {
		if st.online && now.Sub(st.lastSeen) >= timeout {
			st.online = false
			st.downSince = now
			st.downConfirmed = false
			st.registerFlip(now)
			log.Printf("link: peer %s (%s) DOWN (mất tín hiệu > %dms)", ip, st.name, m.timeoutMs)
		}
		if !st.online && !st.downConfirmed && now.Sub(st.downSince) >= confirm {
			st.downConfirmed = true
		}
		if len(st.flips) > 0 {
			cutoff := now.Add(-time.Minute)
			i := 0
			for i < len(st.flips) && st.flips[i].Before(cutoff) {
				i++
			}
			if i > 0 {
				st.flips = st.flips[i:]
			}
		}
		if st.flapping && len(st.flips) <= 6 {
			st.flapping = false
		}
	}
}

func (m *LinkMonitor) flush(force bool) {
	now := time.Now()
	m.mu.Lock()
	report := LinkReport{IP: m.ownIP, Hostname: m.hostname, ServerName: m.ownName}
	dirty := false
	for _, pc := range m.peers {
		st := m.state[pc.IP]
		if st == nil {
			continue
		}
		report.Links = append(report.Links, m.buildLink(pc.IP, st, now))
		if st.dirty() {
			dirty = true
		}
	}
	if !force && (!dirty || now.Sub(m.lastFlush) < time.Second) {
		m.mu.Unlock()
		return
	}
	for _, pc := range m.peers {
		if st := m.state[pc.IP]; st != nil {
			st.reportedOnline = st.online
		}
	}
	m.mu.Unlock()

	select {
	case m.out <- report:
		m.lastFlush = time.Now()
	default:
		log.Printf("link report buffer full, dropping")
	}
}

func (m *LinkMonitor) buildLink(peerIP string, st *peerState, now time.Time) PeerLink {
	link := PeerLink{
		IP:       peerIP,
		Name:     st.name,
		Online:   st.online,
		Flapping: st.flapping,
	}
	if !st.lastSeen.IsZero() {
		link.LastSeen = st.lastSeen.Unix()
	}
	if !st.online && !st.downSince.IsZero() {
		link.DownSince = st.downSince.Unix()
	}
	link.Availability, link.AvailHourStart = m.availability(st, now)
	return link
}

func (m *LinkMonitor) availability(st *peerState, now time.Time) ([]uint32, int64) {
	curHour := now.Unix() / 3600
	startHour := curHour - int64(availHours) + 1
	sec := make(map[int64]uint32)
	for h, s := range st.buckets {
		if h >= startHour {
			sec[h] = s
		}
	}
	if !st.online && !st.downSince.IsZero() {
		addInterval(sec, st.downSince.Unix(), now.Unix())
	}
	arr := make([]uint32, availHours)
	for i := int64(0); i < availHours; i++ {
		arr[i] = sec[startHour+i]
	}
	return arr, startHour * 3600
}

func (m *LinkMonitor) closeDown(st *peerState, now time.Time) {
	if !st.downSince.IsZero() {
		sec := make(map[int64]uint32)
		addInterval(sec, st.downSince.Unix(), now.Unix())
		for h, s := range sec {
			st.buckets[h] += s
		}
		st.downSince = time.Time{}
	}
	cutoff := now.Unix()/3600 - int64(availHours) - 1
	for h := range st.buckets {
		if h < cutoff {
			delete(st.buckets, h)
		}
	}
}

func (st *peerState) registerFlip(now time.Time) {
	st.flips = append(st.flips, now)
	cutoff := now.Add(-time.Minute)
	i := 0
	for i < len(st.flips) && st.flips[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		st.flips = st.flips[i:]
	}
	if len(st.flips) > 6 {
		st.flapping = true
	}
}

func (st *peerState) dirty() bool {
	if st.flapping {
		return false
	}
	if st.online != st.reportedOnline {
		if !st.online {
			return st.downConfirmed
		}
		return true
	}
	return false
}

func addInterval(sec map[int64]uint32, t1, t2 int64) {
	if t2 <= t1 {
		return
	}
	for h := t1 / 3600; h <= (t2-1)/3600; h++ {
		start := h * 3600
		end := start + 3600
		if start < t1 {
			start = t1
		}
		if end > t2 {
			end = t2
		}
		sec[h] += uint32(end - start)
	}
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
