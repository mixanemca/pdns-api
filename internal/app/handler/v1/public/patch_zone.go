package public

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mittwald/go-powerdns/apis/zones"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type internalClient interface {
	FlushAllCache(serverID, name string) error
	AddZone(serverID, zoneType string, bodyBytes []byte) error
}

type ptrrecorder interface {
	AddPTR(ctx context.Context, serverID string, zoneID string, rrset zones.ResourceRecordSet) error
	DelPTR(ctx context.Context, serverID string, zoneID string, rrset zones.ResourceRecordSet) error
}

type PatchZone struct {
	config          config.Config
	ldapZoneDeleter LDAPZoneDeleter
	errorWriter     errorWriter
	stats           stats.PrometheusStatsCollector
	logger          *logrus.Logger
	auth            pdnsApi.Client
	ptrrecorder     ptrrecorder
	internalClient  internalClient
}

func NewPatchZone(config config.Config, ldapZoneDeleter LDAPZoneDeleter, errorWriter errorWriter, stats stats.PrometheusStatsCollector, logger *logrus.Logger, auth pdnsApi.Client, ptrrecorder ptrrecorder, internalClient internalClient) *PatchZone {
	return &PatchZone{config: config, ldapZoneDeleter: ldapZoneDeleter, errorWriter: errorWriter, stats: stats, logger: logger, auth: auth, ptrrecorder: ptrrecorder, internalClient: internalClient}
}

// PatchZone Deletes this zone, all attached metadata and rrsets.
func (s *PatchZone) PatchZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["serverID"]
	zoneID := vars["zoneID"]

	timer := s.stats.GetLabeledResponseTimePeersHistogramTimer(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method)
	defer timer.ObserveDuration()

	decoder := json.NewDecoder(r.Body)
	var z zones.Zone
	err := decoder.Decode(&z)
	if err != nil {
		s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneUpdate, errors.BadRequest.Wrap(err, "decoding input zone"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.BackendTimeout)*time.Second)
	defer cancel()

	for _, rrset := range z.ResourceRecordSets {
		switch rrset.ChangeType {
		case zones.ChangeTypeReplace:
			err = s.auth.Zones().AddRecordSetToZone(ctx, serverID, zoneID, rrset)
			if err != nil {
				s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneUpdate, errors.Wrapf(err, "updating zone %s", zoneID))
				return
			}
			for _, record := range rrset.Records {
				s.logger.WithFields(logrus.Fields{
					"action": log.ActionZoneUpdate,
					"zone":   zoneID,
					"rr":     rrset.Name,
				}).Infof("RR %s was added to zone %s with content %s", rrset.Name, zoneID, record.Content)
				if record.SetPTR {
					// Add new PTR
					err = s.ptrrecorder.AddPTR(ctx, serverID, zoneID, rrset)
					if err != nil {
						s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneUpdate, errors.Wrap(err, "updating revers zone"))
						return
					}
				}
			}
		case zones.ChangeTypeDelete:
			err = s.auth.Zones().RemoveRecordSetFromZone(ctx, serverID, zoneID, rrset.Name, rrset.Type)
			if err != nil {
				s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneUpdate, errors.Wrapf(err, "deleting RR %s from zone %s", rrset.Name, zoneID))
				return
			}
			s.logger.WithFields(logrus.Fields{
				"action": log.ActionZoneUpdate,
				"zone":   zoneID,
				"rr":     rrset.Name,
			}).Infof("RR %s was removed from zone %s", rrset.Name, zoneID)
			// Delete PTR
			err = s.ptrrecorder.DelPTR(ctx, serverID, zoneID, rrset)
			if err != nil {
				s.errorWriter.WriteError(w, r.URL.Path, log.ActionZoneUpdate, errors.Wrapf(err, "deleting PTR %s from zone %s", rrset.Name, zoneID))
				return
			}
		default:
			continue
		}
	}
	// Flush cache
	for _, rr := range z.ResourceRecordSets {
		if err := s.internalClient.FlushAllCache(serverID, rr.Name); err != nil {
			s.errorWriter.WriteError(w, r.URL.Path, log.ActionFlushCache, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
	s.logger.WithFields(logrus.Fields{
		"action": log.ActionZoneDelete,
		"zone":   zoneID,
	}).Infof("Zone %s was deleted", zoneID)
	s.stats.CountCall(s.config.Environment, network.GetHostname(), r.URL.Path, r.Method, http.StatusNoContent)
}
