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
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mixanemca/pdns-api/internal/app/config"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
)

type PatchForwardZoneHandler struct {
	config         config.Config
	errorWriter    errorWriter
	stats          stats.PrometheusStatsCollector
	internalClient internalClient
}

// NewPatchForwardZoneHandler returns new PatchForwardZoneHandler
func NewPatchForwardZoneHandler(config config.Config, errorWriter errorWriter, stats stats.PrometheusStatsCollector, internalClient internalClient) *PatchForwardZoneHandler {
	return &PatchForwardZoneHandler{config: config, errorWriter: errorWriter, stats: stats, internalClient: internalClient}
}

func (s *PatchForwardZoneHandler) PatchForwardZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	zoneID := vars["zoneID"]
	zoneType := vars["zoneType"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = ioutil.ReadAll(r.Body)
	}

	if err := s.internalClient.PatchZone(serverID, zoneType, zoneID, bodyBytes); err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneUpdate, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusNoContent)
}
