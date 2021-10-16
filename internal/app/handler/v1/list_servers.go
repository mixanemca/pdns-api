package v1

import (
	"encoding/json"
	"fmt"
	pdns "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

type ListServersHandler struct {
	config         config.Config
	stats          stats.PrometheusStatsCollector
	powerDNSClient pdns.Client
}

func NewListServersHandler(config config.Config, stats stats.PrometheusStatsCollector, powerDNSClient pdns.Client) *ListServersHandler {
	return &ListServersHandler{config: config, stats: stats, powerDNSClient: powerDNSClient}
}

// ListServers list all servers
func (s *ListServersHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.config.BackendTimeout)*time.Second)
	defer cancel()

	servers, err := s.powerDNSClient.Servers().ListServers(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get all servers list: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(servers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountCall(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
