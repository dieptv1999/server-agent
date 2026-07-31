package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"server-agent/collector"
	"time"
)

type Reporter struct {
	client     *http.Client
	apiURL     string
	secret     string
	serverName string
}

func New(apiURL, secret, serverName string) *Reporter {
	return &Reporter{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		apiURL:     apiURL,
		secret:     secret,
		serverName: serverName,
	}
}

func (r *Reporter) Report(m *collector.Metrics) error {
	if r.serverName != "" {
		m.ServerName = r.serverName
	}

	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", r.apiURL+"/admin/system/server-agent/report", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-server-agent-secret", r.secret)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}
