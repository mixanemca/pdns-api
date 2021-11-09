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
	"net/http"
	"os"

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

type DeleteForwardZonesHandler struct {
	config         config.Config
	stats          statistic.PrometheusStatsCollector
	powerDNSClient pdnsApi.Client
	recursor       pdnsApi.Client
	logger         *logrus.Logger
	errorWriter    errorWriter
	fwzStorage     storage.Storage
}

func NewDeleteForwardZonesHandler(
	config config.Config,
	stats statistic.PrometheusStatsCollector,
	powerDNSClient pdnsApi.Client,
	recursor pdnsApi.Client,
	logger *logrus.Logger,
	errorWriter errorWriter,
	fwzStorage storage.Storage,
) *DeleteForwardZonesHandler {
	return &DeleteForwardZonesHandler{
		config:         config,
		stats:          stats,
		powerDNSClient: powerDNSClient,
		recursor:       recursor,
		logger:         logger,
		errorWriter:    errorWriter,
		fwzStorage:     fwzStorage,
	}
}

// DeleteForwardZonesInternal delete array of forward zones from forward-zones-file
func (s *DeleteForwardZonesHandler) DeleteForwardZonesInternal(w http.ResponseWriter, r *http.Request) {
	// Todo too duplicated code. Need refactoring
	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	// Parse input data to []ForwardZone
	decoder := json.NewDecoder(r.Body)
	var input []forwardzone.ForwardZone

	err := decoder.Decode(&input)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "parsing forward-zones"))
		return
	}

	// Check input data fields
	for _, i := range input {
		if _, err := forwardzone.ParseForwardZoneLine(i.String()); err != nil {
			s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "decoding forward-zones"))
			return
		}
	}

	file, err := os.OpenFile(forwardzone.ForwardZonesFile, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "reading forward-zones-file"))
		return
	}
	defer file.Close()

	fzs, err := forwardzone.ParseForwardZoneFile(file)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "parsing forward-zones-file"))
		return
	}
	// check exists
	for _, inputFZ := range input {
		fzs, err = forwardzone.DeleteForwardZone(fzs, network.Canonicalize(inputFZ.Name))
		if err != nil {
			s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, errors.Wrap(err, "updating forward-zone"))
			return
		}
		continue
	}

	err = s.fwzStorage.Save(fzs)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionForwardZoneDelete, err)
		return
	}

	for _, inputFZ := range input {
		s.logger.WithFields(logrus.Fields{
			"action":       log.ActionForwardZoneDelete,
			"forward-zone": network.DeCanonicalize(inputFZ.Name),
		}).Infof("Forward zone %s was deleted", network.DeCanonicalize(inputFZ.Name))
	}
	w.WriteHeader(http.StatusCreated)
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusCreated)
}
