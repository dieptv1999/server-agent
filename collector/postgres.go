package collector

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type PostgresStatus struct {
	Running           bool   `json:"running"`
	IsMaster          bool   `json:"is_master"`
	SyncState         string `json:"sync_state"`
	ReplicationLagMs  int64  `json:"replication_lag_ms"`
	ActiveConnections int    `json:"active_connections"`
	MaxConnections    int    `json:"max_connections"`
}

type PostgresCollector struct {
	db *sql.DB
}

func NewPostgresCollector(dsn string) *PostgresCollector {
	return &PostgresCollector{}
}

func (p *PostgresCollector) Connect() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	db.SetConnMaxLifetime(30 * time.Second)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	p.db = db
	return db.Ping()
}

func (p *PostgresCollector) Close() {
	if p.db != nil {
		p.db.Close()
	}
}

func (p *PostgresCollector) Collect() *PostgresStatus {
	if p.db == nil {
		return nil
	}
	status := &PostgresStatus{Running: true}

	err := p.db.Ping()
	if err != nil {
		status.Running = false
		log.Printf("pg ping failed: %v", err)
		return status
	}

	var inRecovery bool
	if err := p.db.QueryRow("SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		log.Printf("pg_is_in_recovery failed: %v", err)
	} else {
		status.IsMaster = !inRecovery
	}

	var maxConn int
	if err := p.db.QueryRow("SELECT current_setting('max_connections')::int").Scan(&maxConn); err != nil {
		log.Printf("max_connections failed: %v", err)
	} else {
		status.MaxConnections = maxConn
	}

	var activeConn int
	if err := p.db.QueryRow("SELECT count(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&activeConn); err != nil {
		log.Printf("pg_stat_activity count failed: %v", err)
	} else {
		status.ActiveConnections = activeConn
	}

	if status.IsMaster {
		rows, err := p.db.Query(`
			SELECT sync_state, 
			       COALESCE(EXTRACT(EPOCH FROM replay_lag) * 1000, 0)::bigint 
			FROM pg_stat_replication LIMIT 1`)
		if err != nil {
			log.Printf("pg_stat_replication failed: %v", err)
		} else {
			defer rows.Close()
			if rows.Next() {
				var syncState string
				var lagMs int64
				if err := rows.Scan(&syncState, &lagMs); err != nil {
					log.Printf("scan replication row failed: %v", err)
				} else {
					status.SyncState = syncState
					status.ReplicationLagMs = lagMs
				}
			}
		}
	} else {
		status.SyncState = "replica"
	}

	return status
}
