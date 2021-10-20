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
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone/storage"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	statistic "github.com/mixanemca/pdns-api/internal/infrastructure/stats"
)

type UpdateForwardZoneHandler struct {
	config         config.Config
	stats          statistic.PrometheusStatsCollector
	powerDNSClient pdnsApi.Client
	recursor       pdnsApi.Client
	logger         *logrus.Logger
	errorWriter    errorWriter
	fwzStorage     storage.Storage
}

func NewUpdateForwardZoneHandler(config config.Config, stats statistic.PrometheusStatsCollector, powerDNSClient pdnsApi.Client, recursor pdnsApi.Client, logger *logrus.Logger, errorWriter errorWriter, fwzStorage storage.Storage) *UpdateForwardZoneHandler {
	return &UpdateForwardZoneHandler{config: config, stats: stats, powerDNSClient: powerDNSClient, recursor: recursor, logger: logger, errorWriter: errorWriter, fwzStorage: fwzStorage}
}

// UpdateForwardZoneInternal modifies forward zone in forward-zones-file
func (s *UpdateForwardZoneHandler) UpdateForwardZonesInternal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneID := vars["zoneID"]

	// Todo too duplicated code. Need refactoring
	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	// Parse input data to ForwardZone
	decoder := json.NewDecoder(r.Body)
	var input forwardzone.ForwardZone

	err := decoder.Decode(&input)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneUpdate, errors.Wrap(err, "parsing forward-zones"))
		return
	}

	// Check input data fields
	if _, err := forwardzone.ParseForwardZoneLine(input.String()); err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneUpdate, errors.Wrap(err, "decoding forward-zones"))
		return
	}

	file, err := os.OpenFile(forwardzone.ForwardZonesFile, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneUpdate, errors.Wrap(err, "reading forward-zones-file"))
		return
	}
	defer file.Close()

	fzs, err := forwardzone.ParseForwardZoneFile(file)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneUpdate, errors.Wrap(err, "parsing forward-zones-file"))
		return
	}
	// check exists
	var found bool
	for i, fz := range fzs {
		if network.Canonicalize(zoneID) == network.Canonicalize(fz.Name) {
			fzs[i] = input
			found = true
			break
		}
	}

	// Return 404 if zone not forwarding
	if !found {
		s.logger.WithFields(logrus.Fields{
			"action":       log.ActionForwardZoneUpdate,
			"forward-zone": zoneID,
		}).Warnf("Cannot update zone %s. Zone not forwarding", network.Canonicalize(zoneID))
		http.Error(w, fmt.Sprintf("zone %s not forwarding", zoneID), http.StatusNotFound)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusNotFound)
		return
	}

	err = s.fwzStorage.Save(fzs)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneUpdate, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusNoContent)
}
