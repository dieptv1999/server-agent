package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

const collectTimeout = 3 * time.Second

type ReplicaStatus struct {
	ApplicationName string `json:"application_name"`
	State           string `json:"state"`
	SyncState       string `json:"sync_state"`
	WriteLagMs      int64  `json:"write_lag_ms"`
	FlushLagMs      int64  `json:"flush_lag_ms"`
	ReplayLagMs     int64  `json:"replay_lag_ms"`
}

type ReplicationSlot struct {
	SlotName  string `json:"slot_name"`
	SlotType  string `json:"slot_type"`
	Active    bool   `json:"active"`
	WalStatus string `json:"wal_status"`
	LagMb     int64  `json:"lag_mb"`
}

type PostgresStatus struct {
	Running           bool              `json:"running"`
	IsMaster          bool              `json:"is_master"`
	Replicas          []ReplicaStatus   `json:"replicas"`
	ActiveConnections int               `json:"active_connections"`
	MaxConnections    int               `json:"max_connections"`
	ReplicationSlots  []ReplicationSlot `json:"replication_slots"`
}

type PostgresCollector struct {
	db  *sql.DB
	dsn string
}

func NewPostgresCollector(dsn string) *PostgresCollector {
	return &PostgresCollector{dsn: dsn}
}

func (p *PostgresCollector) Connect() error {
	dsn := p.dsn
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), collectTimeout)
	defer cancel()

	status := &PostgresStatus{Running: true}

	if err := p.db.PingContext(ctx); err != nil {
		status.Running = false
		log.Printf("pg ping failed: %v", err)
		return status
	}

	var inRecovery bool
	if err := p.db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		log.Printf("pg_is_in_recovery failed: %v", err)
	} else {
		status.IsMaster = !inRecovery
	}

	var maxConn int
	if err := p.db.QueryRowContext(ctx, "SELECT current_setting('max_connections')::int").Scan(&maxConn); err != nil {
		log.Printf("max_connections failed: %v", err)
	} else {
		status.MaxConnections = maxConn
	}

	var activeConn int
	if err := p.db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&activeConn); err != nil {
		log.Printf("pg_stat_activity count failed: %v", err)
	} else {
		status.ActiveConnections = activeConn
	}

	if status.IsMaster {
		rows, err := p.db.QueryContext(ctx, `
			SELECT COALESCE(application_name, ''),
			       COALESCE(state, ''),
			       COALESCE(sync_state, ''),
			       COALESCE(EXTRACT(EPOCH FROM write_lag) * 1000, 0)::bigint,
			       COALESCE(EXTRACT(EPOCH FROM flush_lag) * 1000, 0)::bigint,
			       COALESCE(EXTRACT(EPOCH FROM replay_lag) * 1000, 0)::bigint
			FROM pg_stat_replication
			ORDER BY application_name`)
		if err != nil {
			log.Printf("pg_stat_replication failed: %v", err)
		} else {
			func() {
				defer rows.Close()
				var replicas []ReplicaStatus
				for rows.Next() {
					var r ReplicaStatus
					if err := rows.Scan(&r.ApplicationName, &r.State, &r.SyncState, &r.WriteLagMs, &r.FlushLagMs, &r.ReplayLagMs); err != nil {
						log.Printf("scan replication row failed: %v", err)
						continue
					}
					replicas = append(replicas, r)
				}
				status.Replicas = replicas
			}()
		}

		slotRows, err := p.db.QueryContext(ctx, `
			SELECT slot_name,
			       slot_type,
			       active,
			       COALESCE(wal_status, 'unknown'),
			       CASE
			         WHEN slot_type = 'logical' AND confirmed_flush_lsn IS NOT NULL
			           THEN pg_wal_lsn_diff(pg_current_wal_lsn(), confirmed_flush_lsn)::bigint / (1024 * 1024)
			         WHEN restart_lsn IS NOT NULL
			           THEN pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)::bigint / (1024 * 1024)
			         ELSE 0
			       END
			FROM pg_replication_slots
			WHERE NOT temporary
			ORDER BY slot_name`)
		if err != nil {
			log.Printf("pg_replication_slots failed: %v", err)
		} else {
			func() {
				defer slotRows.Close()
				var slots []ReplicationSlot
				for slotRows.Next() {
					var s ReplicationSlot
					if err := slotRows.Scan(&s.SlotName, &s.SlotType, &s.Active, &s.WalStatus, &s.LagMb); err != nil {
						log.Printf("scan replication slot row failed: %v", err)
						continue
					}
					slots = append(slots, s)
				}
				status.ReplicationSlots = slots
			}()
		}
	}

	return status
}
