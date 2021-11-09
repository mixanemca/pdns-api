/*
Copyright © 2021 Michael Bruskov <mixanemca@yandex.ru>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pdns "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"golang.org/x/net/context"
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
	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.config.PDNS.AuthConfig.Timeout)*time.Second)
	defer cancel()

	servers, err := s.powerDNSClient.Servers().ListServers(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get all servers list: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(servers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
