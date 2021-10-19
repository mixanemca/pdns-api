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
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mittwald/go-powerdns/apis/zones"
	"github.com/mittwald/go-powerdns/pdnshttp"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"golang.org/x/net/context"
)

type ZonesHandler struct {
	config         config.Config
	stats          stats.PrometheusStatsCollector
	powerDNSClient pdnsApi.Client
}

func NewZonesHandler(config config.Config, stats stats.PrometheusStatsCollector, powerDNSClient pdnsApi.Client) *ZonesHandler {
	return &ZonesHandler{config: config, stats: stats, powerDNSClient: powerDNSClient}
}

// ListZones list all zones in a server
func (s *ZonesHandler) ListZones(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	zoneID := r.FormValue("zone")

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	var zones []zones.Zone
	var err error

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.config.PDNS.AuthConfig.Timeout)*time.Second)
	defer cancel()

	// Get zone by name from query parameters
	if zoneID != "" {
		zones, err = s.powerDNSClient.Zones().ListZone(ctx, serverID, zoneID)
		if err != nil {
			// 404 Not Found
			if _, ok := err.(pdnshttp.ErrNotFound); ok {
				http.Error(w, fmt.Sprintf("zone %s not found", zoneID), http.StatusNotFound)
				s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusNotFound)
				return
			}
			// 500 Internal Serrver Error
			http.Error(w, fmt.Sprintf("list zone %s: %v", zoneID, err), http.StatusInternalServerError)
			s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
			return
		}
		// 200 OK
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(zones)
		if err != nil {
			http.Error(w, fmt.Sprintf("encoding JSON response: %v", err), http.StatusInternalServerError)
			s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		}
		return
	}

	// Get zones
	zones, err = s.powerDNSClient.Zones().ListZones(ctx, serverID)
	if err != nil {
		http.Error(w, fmt.Sprintf("list zones: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(zones)
	if err != nil {
		http.Error(w, fmt.Sprintf("encoding JSON response: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
	}
	s.stats.CountCall(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}

// ListZone returs zone by name
func (s *ZonesHandler) ListZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	zoneID := vars["zoneID"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.PDNS.AuthConfig.Timeout)*time.Second)
	defer cancel()

	zone, err := s.powerDNSClient.Zones().GetZone(ctx, serverID, zoneID)
	if err != nil {
		// 404 Not Found
		if _, ok := err.(pdnshttp.ErrNotFound); ok {
			http.Error(w, fmt.Sprintf("zone %s not found", zoneID), http.StatusNotFound)
			s.stats.CountCall(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method, http.StatusNotFound)
			return
		}
		// 500 Internal Error
		http.Error(w, fmt.Sprintf("list zone %s: %v", zoneID, err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	// 200 OK
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(zone)
	if err != nil {
		http.Error(w, fmt.Sprintf("encoding JSON response: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountCall(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
