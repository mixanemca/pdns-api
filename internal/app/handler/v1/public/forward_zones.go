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
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"

	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/hashicorp/consul/api"
	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
)

type ForwardZonesHandler struct {
	config         config.Config
	stats          stats.PrometheusStatsCollector
	powerDNSClient pdnsApi.Client
	consulClient   *api.Client
}

func NewForwardZonesHandler(config config.Config, stats stats.PrometheusStatsCollector, powerDNSClient pdnsApi.Client) *ForwardZonesHandler {
	return &ForwardZonesHandler{config: config, stats: stats, powerDNSClient: powerDNSClient}
}

// ListForwardZones returns forwarding zones list
func (s *ForwardZonesHandler) ListForwardZones(w http.ResponseWriter, r *http.Request) {
	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	file, err := os.Open(forwardzone.ForwardZonesFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("reading forward-zones-file: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fzs := make(forwardzone.ForwardZones, 0)
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		s := scanner.Text()
		fz, _ := forwardzone.ParseForwardZoneLine(s)
		if fz != nil {
			fzs = append(fzs, *fz)
		}
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(fzs)
	if err != nil {
		http.Error(w, fmt.Sprintf("encoding forward-zones: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}

// ListForwardZone returns forwarding zone by name
func (s *ForwardZonesHandler) ListForwardZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneID := vars["zoneID"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	file, err := os.Open(forwardzone.ForwardZonesFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("reading forward-zones-file: %v", err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		scnr := scanner.Text()
		fz, _ := forwardzone.ParseForwardZoneLine(scnr)
		if fz.Name == zoneID {
			// 200 OK
			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(fz)
			if err != nil {
				http.Error(w, fmt.Sprintf("encoding forward-zones: %v", err), http.StatusInternalServerError)
				s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
				return
			}
			s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusNotFound)
}
