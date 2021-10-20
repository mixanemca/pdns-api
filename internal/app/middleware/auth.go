package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/ldap"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/sirupsen/logrus"
)

type errorWriter interface {
	WriteError(w http.ResponseWriter, urlPath string, action string, err error)
}

type ldapAuth interface {
	AuthorizeViaLDAP(cnType, zoneType, zone, username string) (bool, error)
}

type authMiddleware struct {
	config      config.Config
	errorWriter errorWriter
	stats       stats.PrometheusStatsCollector
	logger      *logrus.Logger
	ldapAuth    ldapAuth
}

func NewAuthMiddleware(config config.Config, errorWriter errorWriter, stats stats.PrometheusStatsCollector, logger *logrus.Logger, ldapAuth ldapAuth) *authMiddleware {
	return &authMiddleware{config: config, errorWriter: errorWriter, stats: stats, logger: logger, ldapAuth: ldapAuth}
}

func (a *authMiddleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		zoneType := vars["zoneType"]
		zoneID := vars["zoneID"]
		uid := r.Header.Get("X-PDNS-Client-UID")

		if uid == "" {
			w.WriteHeader(http.StatusUnauthorized)
			a.stats.CountError(a.config.Environment, network.GetHostname(), r.URL.Path, http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodPatch, http.MethodPost:
			authorized, err := a.ldapAuth.AuthorizeViaLDAP(ldap.CNTypeReplace, zoneType, zoneID, uid)
			if err != nil {
				a.logger.WithFields(logrus.Fields{
					"action":   log.ActionLDAPAuthorization,
					"zone":     zoneID,
					"zoneType": zoneType,
					"uid":      uid,
				}).Errorf("Failed to authorize user %s for %s: %v", uid, ldap.CNTypeReplace, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				a.stats.CountError(a.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
				return
			}
			if !authorized {
				w.WriteHeader(http.StatusUnauthorized)
				a.stats.CountError(a.config.Environment, network.GetHostname(), r.URL.Path, http.StatusUnauthorized)
				return
			}
		case http.MethodDelete:
			authorized, err := a.ldapAuth.AuthorizeViaLDAP(ldap.CNTypeDelete, zoneType, zoneID, uid)
			if err != nil {
				a.logger.WithFields(logrus.Fields{
					"action":   log.ActionLDAPAuthorization,
					"zone":     zoneID,
					"zoneType": zoneType,
					"uid":      uid,
				}).Errorf("Failed to authorize user %s for %s: %v", uid, ldap.CNTypeDelete, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				a.stats.CountError(a.config.Environment, network.GetHostname(), r.URL.Path, http.StatusInternalServerError)
				return
			}
			if !authorized {
				w.WriteHeader(http.StatusUnauthorized)
				a.stats.CountError(a.config.Environment, network.GetHostname(), r.URL.Path, http.StatusUnauthorized)
				return
			}
		}

		// The show must go on...
		next.ServeHTTP(w, r)
	})
}
