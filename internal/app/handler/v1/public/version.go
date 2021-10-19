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

package public

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
)

type VersionHandler struct {
	config  config.Config
	stats   stats.PrometheusStatsCollector
	Version string `json:"version"`
	Build   string `json:"build"`
	Go      string `json:"go"`
}

func NewVersionHandler(config config.Config, stats stats.PrometheusStatsCollector) *VersionHandler {
	return &VersionHandler{
		config:  config,
		stats:   stats,
		Version: config.Version,
		Build:   config.Build,
		Go:      runtime.Version(),
	}
}

// Get returns json with version info
func (s *VersionHandler) Get(w http.ResponseWriter, r *http.Request) {
	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
