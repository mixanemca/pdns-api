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

package app

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/mixanemca/pdns-api/internal/app/handler/v1/common"
	"github.com/mixanemca/pdns-api/internal/app/handler/v1/private"
	"github.com/mixanemca/pdns-api/internal/app/handler/v1/public"
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
	config         config.Config
	consul         *api.Client
	logger         *logrus.Logger
	internalRouter *mux.Router
	publicRouter   *mux.Router
}

func NewApp(cfg config.Config, logger *logrus.Logger) *app {
	publicRouter := mux.NewRouter()
	internalRouter := mux.NewRouter()

	return &app{
		config:         cfg,
		logger:         logger,
		internalRouter: internalRouter,
		publicRouter:   publicRouter,
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

	recursorPowerDNSClient, err := pdnsApi.New(
		pdnsApi.WithBaseURL(a.config.PDNS.RecursorConfig.BaseURL),
		pdnsApi.WithAPIKeyAuthentication(a.config.PDNS.RecursorConfig.ApiKey),
	)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a PowerDNS Authoritative API client: %v", err)
	}

	_, err = consul.NewConsulClient(a.config)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a Consul API client: %v", err)
	}

	a.consul, err = api.NewClient(api.DefaultConfig())
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a Consul API client: %v", err)
	}

	prometheusStats := a.initStats()

	errorWriter := network.NewErrorWriter(a.config, a.logger, prometheusStats)
	compositeFZStorage := a.createCompositeStorage()

	healthHandler := common.NewHealthHandler(a.config)
	listServersHandler := public.NewListServersHandler(a.config, prometheusStats, authPowerDNSClient)
	listServerHandler := public.NewListServerHandler(a.config, prometheusStats, authPowerDNSClient)
	searchDataHandler := public.NewListServerHandler(a.config, prometheusStats, authPowerDNSClient)
	forwardZonesHandler := public.NewForwardZonesHandler(a.config, prometheusStats, authPowerDNSClient)
	zonesHandler := public.NewZonesHandler(a.config, prometheusStats, authPowerDNSClient)
	versionHandler := public.NewVersionHandler(a.config, prometheusStats)

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

	flushHandler := private.NewFlushHandler(a.config, prometheusStats, authPowerDNSClient, recursorPowerDNSClient, a.logger)
	internalAddForwardZoneHandler := private.NewAddForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	deleteForwardZoneHandler := private.NewDeleteForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	deleteForwardZonesHandler := private.NewDeleteForwardZonesHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	updateForwardZonesHandler := private.NewUpdateForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)

	// HTTP internal Handlers
	a.internalRouter.HandleFunc("/api/v1/health", healthHandler.Health).Methods(http.MethodGet)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/cache/flush", flushHandler.FlushInternal).Methods(http.MethodPut)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones", internalAddForwardZoneHandler.AddForwardZonesInternal).Methods(http.MethodPost)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones", deleteForwardZonesHandler.DeleteForwardZonesInternal).Methods(http.MethodDelete)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones/{zoneID}", updateForwardZonesHandler.UpdateForwardZonesInternal).Methods(http.MethodPatch)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones/{zoneID}", deleteForwardZoneHandler.DeleteForwardZoneInternal).Methods(http.MethodDelete)

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

	addZoneHanler := public.NewAddZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
	)

	deleteZoneHanler := public.NewDeleteZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
	)

	patchZoneHanler := public.NewPatchZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
		ptrRecorder,
		internalClient,
	)
	publicAddForwardZoneHandler := public.NewAddForwardZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
		internalClient,
	)

	if viper.GetBool("ldap.enabled") {
		authRouter := a.publicRouter.Methods(http.MethodDelete, http.MethodPatch, http.MethodPost).Subrouter()
		authRouter.Use(authMiddleware.AuthMiddleware)
		// HTTP Handlers with Authorization
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicAddForwardZoneHandler.AddForwardZone).Methods(http.MethodPost)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}", addZoneHanler.AddZone).Methods(http.MethodPost)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", patchZoneHanler.PatchZone).Methods(http.MethodPatch)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", deleteZoneHanler.DeleteZone).Methods(http.MethodDelete)
	} else {
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicAddForwardZoneHandler.AddForwardZone).Methods(http.MethodPost)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}", addZoneHanler.AddZone).Methods(http.MethodPost)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", patchZoneHanler.PatchZone).Methods(http.MethodPatch)
		a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", deleteZoneHanler.DeleteZone).Methods(http.MethodDelete)
	}

	// HTTP Server
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

	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="0.1"} 1
	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="0.25"} 1
	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="0.5"} 1
	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="1"} 1
	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="2.5"} 1
	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="5"} 1
	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="10"} 1
	// pdns_api_response_time_s_bucket{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s.",le="+Inf"} 1
	// pdns_api_response_time_s_sum{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s."} 0.000125775
	// pdns_api_response_time_s_count{node="pdns-dev01",method="GET",path="/api/v1/servers/localhost/forward-zones/omega.k8s."} 1
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
