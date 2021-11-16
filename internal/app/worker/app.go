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

package worker

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	pdnsApi "github.com/mittwald/go-powerdns"
	v12 "github.com/mixanemca/pdns-api/internal/app/common/handler/v1"
	v1 "github.com/mixanemca/pdns-api/internal/app/worker/handler/v1"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone/storage"
	"github.com/mixanemca/pdns-api/internal/infrastructure/client"
	"github.com/mixanemca/pdns-api/internal/infrastructure/consul"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/context"

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

	logger.Debug("Create new Worker app")

	return &app{
		config:         cfg,
		logger:         logger,
		internalRouter: internalRouter,
		publicRouter:   publicRouter,
	}
}

//The entry point of pdns-api
func (a *app) Run(withHealth bool) {
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

	a.consul, err = consul.NewConsulClient(a.config)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a Consul API client: %v", err)
	}
	// Create a service for pdns-api-internal
	internalService, err := connect.NewService(client.PDNSInternalServiceName, a.consul)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a Consul Connect service %s: %v", client.PDNSInternalServiceName, err)
	}

	prometheusStats := a.initStats()

	errorWriter := network.NewErrorWriter(a.config, a.logger, prometheusStats)
	compositeFZStorage := a.createCompositeStorage()

	// Prometheus metrics
	a.publicRouter.Handle("/metrics", promhttp.Handler())

	flushHandler := v1.NewFlushHandler(a.config, prometheusStats, authPowerDNSClient, recursorPowerDNSClient, a.logger)
	internalAddForwardZoneHandler := v1.NewAddForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	deleteForwardZoneHandler := v1.NewDeleteForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	deleteForwardZonesHandler := v1.NewDeleteForwardZonesHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	updateForwardZonesHandler := v1.NewUpdateForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)

	// HTTP internal Handlers
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/cache/flush", flushHandler.FlushInternal).Methods(http.MethodPut)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones", internalAddForwardZoneHandler.AddForwardZonesInternal).Methods(http.MethodPost)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones", deleteForwardZonesHandler.DeleteForwardZonesInternal).Methods(http.MethodDelete)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones/{zoneID}", updateForwardZonesHandler.UpdateForwardZonesInternal).Methods(http.MethodPatch)
	a.internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones/{zoneID}", deleteForwardZoneHandler.DeleteForwardZoneInternal).Methods(http.MethodDelete)

	// Internal HTTP Server
	internalAddr := net.JoinHostPort(a.config.Internal.Address, a.config.Internal.Port)

	internalHTTPServer := &http.Server{
		Addr:      internalAddr,
		Handler:   a.internalRouter,
		TLSConfig: internalService.ServerTLSConfig(),
	}
	go func() {
		if err := internalHTTPServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			a.logger.WithFields(logrus.Fields{
				"action": log.ActionSystem,
			}).Fatalf("error occurred while running internal http server: %s\n", err.Error())
		}
	}()

	if withHealth {
		a.startHealthServer()
	}

	a.logger.Infof("Internal HTTP server started and listen on %s", net.JoinHostPort(a.config.Internal.Address, a.config.Internal.Port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.config.Internal.Timeout.Read)*time.Second)
	defer cancel()
	if err := internalHTTPServer.Shutdown(ctx); err != nil {
		a.logger.Errorf("Stopping internal HTTP server: %v", err)
	}
	a.logger.Info("Internal HTTP server stopped")
}

func (a *app) startHealthServer() {
	healthHandler := v12.NewHealthHandler(a.config)
	a.publicRouter.HandleFunc("/api/v1/health", healthHandler.Health).Methods(http.MethodGet)
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
			}).Fatalf("error occurred while running public http server: %s\n", err.Error())
		}
	}()
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
