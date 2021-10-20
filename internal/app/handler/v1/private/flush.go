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

package private

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mittwald/go-powerdns/pdnshttp"
	"github.com/mixanemca/pdns-api/internal/app/config"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	statistic "github.com/mixanemca/pdns-api/internal/infrastructure/stats"
)

type FlushHandler struct {
	config         config.Config
	stats          statistic.PrometheusStatsCollector
	powerDNSClient pdnsApi.Client
	recursor       pdnsApi.Client
	logger         *logrus.Logger
}

func NewFlushHandler(config config.Config, stats statistic.PrometheusStatsCollector, powerDNSClient pdnsApi.Client, recursor pdnsApi.Client, logger *logrus.Logger) *FlushHandler {
	return &FlushHandler{config: config, stats: stats, powerDNSClient: powerDNSClient, recursor: recursor, logger: logger}
}

// Flush Flush a cache-entry by name
func (s *FlushHandler) FlushInternal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	domain := r.FormValue("domain")

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	// Authoritative
	authResult, err := s.powerDNSClient.Cache().Flush(context.Background(), serverID, domain)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
			"rr":     network.DeCanonicalize(domain),
		}).Error(err.Error())
		err := json.NewEncoder(w).Encode(err)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(err.(pdnshttp.ErrUnexpectedStatus).StatusCode)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, err.(pdnshttp.ErrUnexpectedStatus).StatusCode)
		return
	}
	// Recursive
	recResult, err := s.recursor.Cache().Flush(context.Background(), serverID, domain)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"action": log.ActionFlushCache,
			"rr":     network.DeCanonicalize(domain),
		}).Error(err.Error())
		err := json.NewEncoder(w).Encode(err)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(err.(pdnshttp.ErrUnexpectedStatus).StatusCode)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, err.(pdnshttp.ErrUnexpectedStatus).StatusCode)
		return
	}
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionFlushCache,
		"rr":     network.DeCanonicalize(domain),
	}).Infof("%s for %s", log.ActionFlushCache, network.DeCanonicalize(domain))
	authResult.Count += recResult.Count
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(authResult)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
		return
	}
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusOK)
}
