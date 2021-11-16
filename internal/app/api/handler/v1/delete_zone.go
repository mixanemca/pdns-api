package v1

import (
	"fmt"
	"net/http"
	"time"

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
	"golang.org/x/net/context"
)

type errorWriter interface {
	WriteError(w http.ResponseWriter, urlPath string, action string, err error)
}

type DeleteZone struct {
	config          config.Config
	ldapZoneDeleter ldap.LDAPZoneDeleter
	errorWriter     errorWriter
	stats           stats.PrometheusStatsCollector
	logger          *logrus.Logger
	auth            pdnsApi.Client
}

func NewDeleteZone(config config.Config, ldapZoneDeleter ldap.LDAPZoneDeleter, errorWriter errorWriter, stats stats.PrometheusStatsCollector, logger *logrus.Logger, auth pdnsApi.Client) *DeleteZone {
	return &DeleteZone{config: config, ldapZoneDeleter: ldapZoneDeleter, errorWriter: errorWriter, stats: stats, logger: logger, auth: auth}
}

// DeleteZone Deletes this zone, all attached metadata and rrsets.
func (s *DeleteZone) DeleteZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	zoneID := vars["zoneID"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	// Delete zone from LDAP
	if viper.GetBool("ldap.enabled") {
		if err := s.ldapZoneDeleter.LDAPDelZone(forwardzone.ZoneTypeZone, zoneID); err != nil {
			s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneDelete, err)
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.PDNS.AuthConfig.Timeout)*time.Second)
	defer cancel()

	err := s.auth.Zones().DeleteZone(ctx, serverID, zoneID)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneDelete, errors.Wrapf(err, "deleting zone %s", zoneID))
		return
	}

	// Delete zone from forward-zones-file
	client := &http.Client{}
	url := fmt.Sprintf("http://127.0.0.1:8080/api/v1/servers/%s/forward-zones/%s", serverID, zoneID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneDelete, errors.Wrapf(err, "make a request for delete forward-zone %s", zoneID))
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneDelete, errors.Wrapf(err, "doing a request for delete forward-zone %s", zoneID))
		return
	}
	resp.Body.Close()

	w.WriteHeader(http.StatusNoContent)
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusCreated)

	w.WriteHeader(http.StatusNoContent)
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionZoneDelete,
		"zone":   zoneID,
	}).Infof("Zone %s was deleted", zoneID)
	s.stats.CountError(s.config.Environment, network.GetHostname(), r.URL.Path, http.StatusNoContent)
}
