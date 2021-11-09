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

type DeleteForwardZoneHandler struct {
	config      config.Config
	stats       statistic.PrometheusStatsCollector
	auth        pdnsApi.Client
	recursor    pdnsApi.Client
	logger      *logrus.Logger
	errorWriter errorWriter
	fwzStorage  storage.Storage
}

func NewDeleteForwardZoneHandler(
	config config.Config,
	stats statistic.PrometheusStatsCollector,
	auth pdnsApi.Client,
	recursor pdnsApi.Client,
	logger *logrus.Logger,
	errorWriter errorWriter,
	fwzStorage storage.Storage,
) *DeleteForwardZoneHandler {
	return &DeleteForwardZoneHandler{
		config:      config,
		stats:       stats,
		auth:        auth,
		recursor:    recursor,
		logger:      logger,
		errorWriter: errorWriter,
		fwzStorage:  fwzStorage,
	}
}

// DeleteForwardZoneInternal delete array of forward zones from forward-zones-file
func (s *DeleteForwardZoneHandler) DeleteForwardZoneInternal(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	zoneID := vars["zoneID"]

	// Todo too duplicated code. Need refactoring
	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	file, err := os.OpenFile(forwardzone.ForwardZonesFile, os.O_RDWR, 0644)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "reading forward-zones-file"))
		return
	}
	defer file.Close()

	fzs, err := forwardzone.ParseForwardZoneFile(file)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "decoding forward-zones"))
		return
	}

	// Iterate by forward zones, delete if request zone found
	var found bool
	for i, fz := range fzs {
		if network.Canonicalize(zoneID) == network.Canonicalize(fz.Name) {
			fzs = append(fzs[:i], fzs[i+1:]...)
			found = true
			break
		}
	}

	// Return 404 if forward-zone not found
	if !found {
		s.logger.WithFields(logrus.Fields{
			"action":       log.ActionForwardZoneDelete,
			"forward-zone": zoneID,
		}).Warnf("Cannot delete zone. Zone %s not forwarding", network.Canonicalize(zoneID))
		http.Error(w, fmt.Sprintf("zone %s not forwarding", zoneID), http.StatusNotFound)
		s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusNotFound)
		return
	}

	err = s.fwzStorage.Save(fzs)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, err)
		return
	}

	s.logger.WithFields(logrus.Fields{
		"action":       log.ActionForwardZoneDelete,
		"forward-zone": zoneID,
	}).Infof("forwarding zone %s deleted", network.Canonicalize(zoneID))
	w.WriteHeader(http.StatusOK)
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusOK)
}
