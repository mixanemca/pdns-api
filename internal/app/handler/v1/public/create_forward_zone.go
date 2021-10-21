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
	"bytes"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	pdnsApi "github.com/mittwald/go-powerdns"
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

type AddForwardZoneHandler struct {
	config         config.Config
	ldapZoneAdder  ldap.LDAPZoneAdder
	errorWriter    errorWriter
	stats          stats.PrometheusStatsCollector
	logger         *logrus.Logger
	auth           pdnsApi.Client
	internalClient internalClient
}

func NewAddForwardZone(config config.Config, ldapZoneAdder ldap.LDAPZoneAdder, errorWriter errorWriter, stats stats.PrometheusStatsCollector, logger *logrus.Logger, auth pdnsApi.Client, internalClient internalClient) *AddForwardZoneHandler {
	return &AddForwardZoneHandler{config: config, ldapZoneAdder: ldapZoneAdder, errorWriter: errorWriter, stats: stats, logger: logger, auth: auth, internalClient: internalClient}
}

// AddForwardZone creates a new forwarding zone
func (s *AddForwardZoneHandler) AddForwardZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	zoneType := vars["zoneType"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	var bodyBytes []byte
	var data bytes.Buffer
	if r.Body != nil {
		bodyBytes, _ = ioutil.ReadAll(r.Body)
		data.Write(bodyBytes)
	}

	fzsInput, err := forwardzone.ParseForwardZonesInput(&data)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneAdd, err)
		return
	}

	file, err := os.OpenFile(forwardzone.ForwardZonesFile, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneAdd, errors.Wrap(err, "reading forward-zones-file"))
		return
	}
	defer file.Close()

	fzsActual, err := forwardzone.ParseForwardZoneFile(file)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneAdd, errors.Wrap(err, "parsing forward-zones-file"))
		return
	}

	// Check ForwardZone is already exists
	for _, fzInput := range fzsInput {
		for _, fz := range fzsActual {
			if network.Canonicalize(fz.Name) == network.Canonicalize(fzInput.Name) {
				s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneAdd, errors.Conflict.Newf("forward-zone %s already exists", fz.Name))
				return
			}
		}
	}

	if viper.GetBool("ldap.enabled") {
		for _, inputFZ := range fzsInput {
			// Create forward-zone in LDAP
			if err := s.ldapZoneAdder.LDAPAddZone(forwardzone.ZoneTypeForwardZone, inputFZ.Name); err != nil {
				s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneAdd, err)
				return
			}
		}
	}

	if err = s.internalClient.AddZone(serverID, zoneType, bodyBytes); err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneAdd, err)
		return
	}

	// OK
	w.WriteHeader(http.StatusCreated)
	s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusCreated)
}
