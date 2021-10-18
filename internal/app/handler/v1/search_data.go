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
	"strconv"
	"time"

	"github.com/gorilla/mux"
	pdns "github.com/mittwald/go-powerdns"
	"github.com/mittwald/go-powerdns/apis/search"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	pdns2 "github.com/mixanemca/pdns-api/internal/pdns"
	"golang.org/x/net/context"
)

type SearchDataHandler struct {
	config         config.Config
	stats          stats.PrometheusStatsCollector
	powerDNSClient pdns.Client
}

func NewSearchDataHandler(config config.Config, stats stats.PrometheusStatsCollector, powerDNSClient pdns.Client) *ListServersHandler {
	return &ListServersHandler{config: config, stats: stats, powerDNSClient: powerDNSClient}
}

// SearchData lists all known servers
func (s *ListServersHandler) SearchData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	var max string
	var ot search.ObjectType
	var m int
	var err error

	q := r.URL.Query()
	query := q.Get("q")
	if len(q) == 0 || query == "" {
		http.Error(w, "not enough query parameters", http.StatusBadRequest)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusBadRequest)
		return
	}
	if max = q.Get("max"); max == "" {
		max = pdns2.DefaultMaxResults
	}
	if m, err = strconv.Atoi(max); err != nil {
		http.Error(w, fmt.Sprintf("bad 'max' query parameter: %v", err), http.StatusBadRequest)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusBadRequest)
		return
	}
	objectType := q.Get("object_type")
	switch objectType {
	case "zone":
		ot = search.ObjectTypeZone
	case "record":
		ot = search.ObjectTypeRecord
	case "comment":
		ot = search.ObjectTypeComment
	default:
		ot = search.ObjectTypeAll
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.BackendTimeout)*time.Second)
	defer cancel()

	result, err := s.powerDNSClient.Search().Search(ctx, serverID, query, m, ot)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to search by query %s: %v", query, err), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountCall(s.config.Environment, infrastructure.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
