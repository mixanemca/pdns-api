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
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	"github.com/mixanemca/pdns-api/internal/infrastructure/ldap"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type DelForwardZoneHandler struct {
	config          config.Config
	ldapZoneDeleter ldap.LDAPZoneDeleter
	errorWriter     errorWriter
	stats           stats.PrometheusStatsCollector
	logger          *logrus.Logger
	internalClient  internalClient
}

// NewDelForwardZoneHandler returns new DelForwardZoneHandler
func NewDelForwardZoneHandler(config config.Config, ldapZoneDeleter ldap.LDAPZoneDeleter, errorWriter errorWriter, stats stats.PrometheusStatsCollector, logger *logrus.Logger, internalClient internalClient) *DelForwardZoneHandler {
	return &DelForwardZoneHandler{config: config, ldapZoneDeleter: ldapZoneDeleter, errorWriter: errorWriter, stats: stats, logger: logger, internalClient: internalClient}
}

func (s *DelForwardZoneHandler) DelForwardZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	zoneID := vars["zoneID"]
	zoneType := vars["zoneType"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	file, err := os.OpenFile(forwardzone.ForwardZonesFile, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "reading forward-zones-file"))
		return
	}
	defer file.Close()

	fzsActual, err := forwardzone.ParseForwardZoneFile(file)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "parsing forward-zones-file"))
		return
	}

	// check that fz exists
	found := false
	for _, inputFZ := range fzsActual {
		if network.Canonicalize(zoneID) == network.Canonicalize(inputFZ.Name) {
			found = true
			break
		}
	}
	// 404 Not Found
	if !found {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.NotFound.New("forward-zone not found"))
		return
	}

	if viper.GetBool("ldap.enabled") {
		if err := s.ldapZoneDeleter.LDAPDelZone(forwardzone.ZoneTypeForwardZone, zoneID); err != nil {
			s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, err)
			return
		}
	}

	if err := s.internalClient.DelZone(serverID, zoneType, zoneID); err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, err)
		return
	}

	// OK
	w.WriteHeader(http.StatusOK)
	s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
