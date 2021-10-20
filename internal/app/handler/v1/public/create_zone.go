package public

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mittwald/go-powerdns/apis/zones"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/domain/zone"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

type AddZone struct {
	config          config.Config
	ldapZoneDeleter LDAPZoneDeleter
	errorWriter     errorWriter
	stats           stats.PrometheusStatsCollector
	logger          *logrus.Logger
	auth            pdnsApi.Client
}

func NewAddZone(config config.Config, ldapZoneDeleter LDAPZoneDeleter, errorWriter errorWriter, stats stats.PrometheusStatsCollector, logger *logrus.Logger, auth pdnsApi.Client) *AddZone {
	return &AddZone{config: config, ldapZoneDeleter: ldapZoneDeleter, errorWriter: errorWriter, stats: stats, logger: logger, auth: auth}
}

// CreateZone creates a new domain, returns the Zone on creation.
func (s *AddZone) AddZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = ioutil.ReadAll(r.Body)
	}

	decoder := json.NewDecoder(ioutil.NopCloser(bytes.NewReader(bodyBytes)))

	var input zones.Zone
	err := decoder.Decode(&input)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneAdd, errors.BadRequest.Wrap(err, "decoding input zone"))
		return
	}

	// Create zone from LDAP
	if viper.GetBool("ldap.enabled") {
		if err := s.ldapZoneDeleter.LDAPDelZone(forwardzone.ZoneTypeZone, input.Name); err != nil {
			s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneAdd, err)
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.BackendTimeout)*time.Second)
	defer cancel()

	createdZone, err := s.auth.Zones().CreateZone(ctx, serverID, input)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneAdd, errors.Wrapf(err, "creating zone %s", input.Name))
		return
	}

	// Add zone to forwarder
	var fz = forwardzone.ForwardZone{
		Name:        input.Name,
		Nameservers: []string{zone.LocalNameserver},
	}
	client := &http.Client{}
	url := fmt.Sprintf("http://127.0.0.1:8080/api/v1/servers/%s/forward-zones", serverID)
	b, err := json.Marshal(fz)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneAdd, errors.Wrapf(err, "marshaling forward-zone %s", input.Name))
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneAdd, errors.Wrapf(err, "make a request for add forward-zone %s", input.Name))
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneAdd, errors.Wrapf(err, "do request for add forward-zone %s", input.Name))
		return
	}
	resp.Body.Close()

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(createdZone)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneAdd, errors.Wrap(err, "encoding JSON response"))
		return
	}
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionZoneAdd,
		"zone":   fz.Name,
	}).Infof("Zone %s was created with nameservers %s", fz.Name, strings.Join(fz.Nameservers, ","))
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusCreated)
}
