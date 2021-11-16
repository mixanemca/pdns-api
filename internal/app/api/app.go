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

package api

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	apiV1 "github.com/mixanemca/pdns-api/internal/app/api/handler/v1"
	commonV1 "github.com/mixanemca/pdns-api/internal/app/common/handler/v1"
	"github.com/mixanemca/pdns-api/internal/app/middleware"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone/storage"
	"github.com/mixanemca/pdns-api/internal/domain/zone"
	"github.com/mixanemca/pdns-api/internal/infrastructure/client"
	"github.com/mixanemca/pdns-api/internal/infrastructure/ldap"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/spf13/viper"

	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/infrastructure/consul"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gorilla/mux"
	"github.com/mixanemca/pdns-api/internal/app/config"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

type app struct {
	config       config.Config
	consul       *api.Client
	logger       *logrus.Logger
	publicRouter *mux.Router
}

func NewApp(cfg config.Config, logger *logrus.Logger) *app {
	publicRouter := mux.NewRouter()

	return &app{
		config:       cfg,
		logger:       logger,
		publicRouter: publicRouter,
	}
}

//The entry point of pdns-api
func (a *app) Run() {
	authPowerDNSClient, err := pdnsApi.New(
		pdnsApi.WithBaseURL(a.config.PDNS.AuthConfig.BaseURL),
		pdnsApi.WithAPIKeyAuthentication(a.config.PDNS.AuthConfig.ApiKey),
	)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a PowerDNS Authoritative API client: %v", err)
	}

	a.consul, err = consul.NewConsulClient(a.config)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a Consul API client: %v", err)
	}

	prometheusStats := a.initStats()

	errorWriter := network.NewErrorWriter(a.config, a.logger, prometheusStats)

	healthHandler := commonV1.NewHealthHandler(a.config)
	listServersHandler := apiV1.NewListServersHandler(a.config, errorWriter, prometheusStats, a.logger, authPowerDNSClient)
	listServerHandler := apiV1.NewListServerHandler(a.config, prometheusStats, authPowerDNSClient)
	searchDataHandler := apiV1.NewListServerHandler(a.config, prometheusStats, authPowerDNSClient)
	forwardZonesHandler := apiV1.NewForwardZonesHandler(a.config, prometheusStats, authPowerDNSClient)
	zonesHandler := apiV1.NewZonesHandler(a.config, prometheusStats, authPowerDNSClient)
	versionHandler := apiV1.NewVersionHandler(a.config, prometheusStats)

	// HTTP public Handlers
	a.publicRouter.HandleFunc("/api/v1/health", healthHandler.Health).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers", listServersHandler.ListServers).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}", listServerHandler.ListServer).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/search-data", searchDataHandler.SearchData).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/forward-zones", forwardZonesHandler.ListForwardZones).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/forward-zones/{zoneID}", forwardZonesHandler.ListForwardZone).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/zones", zonesHandler.ListZones).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/zones/{zoneID}", zonesHandler.ListZone).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/version", versionHandler.Get).Methods(http.MethodGet)

	// Prometheus metrics
	a.publicRouter.Handle("/metrics", promhttp.Handler())

	// Create a service for pdns-api-internal
	internalService, err := connect.NewService(client.PDNSInternalServiceName, a.consul)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a Consul Connect service %s: %v", client.PDNSInternalServiceName, err)
	}

	ldapService, err := ldap.NewLDAPService(a.logger, a.config)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a ldap auth client: %v", err)
	}

	ptrRecorder := zone.NewPTR(a.logger, authPowerDNSClient)
	internalClient := client.NewClient(
		a.config,
		a.consul,
		internalService,
	)

	authMiddleware := middleware.NewAuthMiddleware(
		a.config,
		errorWriter,
		prometheusStats,
		a.logger,
		ldapService,
	)

	addZoneHanler := apiV1.NewAddZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
	)

	deleteZoneHanler := apiV1.NewDeleteZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
	)

	patchZoneHanler := apiV1.NewPatchZone(
		a.config,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
		ptrRecorder,
		internalClient,
	)
	publicAddForwardZonesHandler := apiV1.NewAddForwardZonesHandler(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		internalClient,
	)
	publicDelForwardZonesHandler := apiV1.NewDelForwardZonesHandler(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		internalClient,
	)
	publicDelForwardZoneHandler := apiV1.NewDelForwardZoneHandler(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		internalClient,
	)
	publicPatchForwardZoneHandler := apiV1.NewPatchForwardZoneHandler(
		a.config,
		errorWriter,
		prometheusStats,
		internalClient,
	)

	if viper.GetBool("ldap.enabled") {
		authRouter := a.publicRouter.Methods(http.MethodDelete, http.MethodPatch, http.MethodPost).Subrouter()
		authRouter.Use(authMiddleware.AuthMiddleware)
		// HTTP Handlers with Authorization
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicAddForwardZonesHandler.AddForwardZones).Methods(http.MethodPost)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicDelForwardZonesHandler.DelForwardZones).Methods(http.MethodDelete)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicPatchForwardZoneHandler.PatchForwardZone).Methods(http.MethodPatch)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicDelForwardZoneHandler.DelForwardZone).Methods(http.MethodDelete)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}", addZoneHanler.AddZone).Methods(http.MethodPost)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", patchZoneHanler.PatchZone).Methods(http.MethodPatch)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", deleteZoneHanler.DeleteZone).Methods(http.MethodDelete)
	} else {
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicAddForwardZonesHandler.AddForwardZones).Methods(http.MethodPost)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicDelForwardZonesHandler.DelForwardZones).Methods(http.MethodDelete)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicPatchForwardZoneHandler.PatchForwardZone).Methods(http.MethodPatch)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicDelForwardZoneHandler.DelForwardZone).Methods(http.MethodDelete)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}", addZoneHanler.AddZone).Methods(http.MethodPost)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", patchZoneHanler.PatchZone).Methods(http.MethodPatch)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", deleteZoneHanler.DeleteZone).Methods(http.MethodDelete)
	}

	// Public HTTP Server
	publicAddr := net.JoinHostPort(a.config.PublicHTTP.Address, a.config.PublicHTTP.Port)

	publicHTTPServer := &http.Server{
		Addr:    publicAddr,
		Handler: a.publicRouter,
	}
	go func() {
		if err := publicHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.WithFields(logrus.Fields{
				"action": log.ActionSystem,
			}).Fatalf("error occurred while running http server: %s\n", err.Error())
		}
	}()

	a.logger.Infof("Version: %s; Build: %s", a.config.Version, a.config.Build)
	a.logger.Infof("Server started and listen on %s", net.JoinHostPort(a.config.PublicHTTP.Address, a.config.PublicHTTP.Port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	<-quit

	a.logger.Info("Server stopped")
}

func (a *app) createCompositeStorage() storage.Storage {
	fsStorage := storage.NewFSStorage(forwardzone.ForwardZonesFile)
	consulStorage := storage.NewConsuleStorage(a.consul)
	return storage.NewCompositeStorage([]storage.Storage{fsStorage, consulStorage})
}

func (a *app) initStats() *stats.PrometheusStats {
	// pdns_api_up{dc="dataspace",environment="dev",instance="pdns-dev01:443",job="pdns-api",node="pdns-dev01"}
	// 1 if the instance is healthy, i.e. reachable, or 0 if the scrape failed
	pdnsUp := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pdns_api_up",
			Help: "Whether the pdns-api server is up",
		},
		[]string{
			"environment",
			"dc",
			"node",
		},
	)

	// requests counter
	// pdns_api_total{code="200",node="pdns-dev01",method="GET",uri="/api/v1/health"} 525
	pdnsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pdns_api_total",
			Help: "Statistics of calls endpoints",
		},
		[]string{
			"environment",
			"node",
			"path",
			"method",
			"code",
		},
	)

	pdnsResponseTimeHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pdns_api_response_time_s",
			Help:    "Histogram of response times in seconds",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{
			"environment",
			"node",
			"path",
			"method",
		},
	)

	// pdns_api_errors_total{code="400",node="pdns-dev01",path="/api/v1/servers/localhost/cache/flush"} 1
	pdnsErrorsCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pdns_api_errors_total",
			Help: "Statistics of errors per instance",
		},
		[]string{
			"environment",
			"node",
			"path",
			"code",
		},
	)

	prometheus.MustRegister(pdnsUp)
	prometheus.MustRegister(pdnsCounter)
	prometheus.MustRegister(pdnsErrorsCounter)
	prometheus.MustRegister(pdnsResponseTimeHistogram)

	return stats.NewPrometheusStats(pdnsUp, pdnsCounter, pdnsErrorsCounter, pdnsResponseTimeHistogram)
}
