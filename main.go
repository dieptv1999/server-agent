package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"server-agent/collector"
	"server-agent/reporter"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	envPath := flag.String("env", ".env", "path to .env file")
	flag.Parse()
	godotenv.Load(*envPath)

	apiURL := os.Getenv("API_URL")
	secret := os.Getenv("SECRET")
	serverName := os.Getenv("SERVER_NAME")

	if apiURL == "" || secret == "" {
		log.Fatal("API_URL and SECRET are required")
	}

	r := reporter.New(apiURL, secret, serverName)

	pgDSN := os.Getenv("DATABASE_URL")
	var pgCollector *collector.PostgresCollector
	if pgDSN != "" {
		pgCollector = collector.NewPostgresCollector(pgDSN)
		if err := pgCollector.Connect(); err != nil {
			log.Printf("Postgres connection failed, PG monitoring disabled: %v", err)
			pgCollector = nil
		} else {
			defer pgCollector.Close()
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("server-agent started, reporting every 5s")

	report := func() {
		metrics := collector.CollectAll()
		if pgCollector != nil {
			metrics.Postgres = pgCollector.Collect()
		}
		if err := r.Report(metrics); err != nil {
			log.Printf("report error: %v", err)
		}
	}

	report()

	for {
		select {
		case <-ticker.C:
			report()
		case <-sigCh:
			log.Println("shutting down...")
			return
		}
	}
}
