package v1

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	pdns "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

type ListServerHandler struct {
	config         config.Config
	stats          stats.PrometheusStatsCollector
	powerDNSClient pdns.Client
}

func NewListServerHandler(config config.Config, stats stats.PrometheusStatsCollector, powerDNSClient pdns.Client) *ListServersHandler {
	return &ListServersHandler{config: config, stats: stats, powerDNSClient: powerDNSClient}
}

// ListServer list all servers
func (s *ListServersHandler) ListServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.config.BackendTimeout)*time.Second)
	defer cancel()

	server, err := s.powerDNSClient.Servers().GetServer(ctx, serverID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get all servers list: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(server)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountCall(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
