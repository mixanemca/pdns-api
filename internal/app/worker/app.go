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

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	pdnsApi "github.com/mittwald/go-powerdns"
	commonV1 "github.com/mixanemca/pdns-api/internal/app/common/handler/v1"
	workerV1 "github.com/mixanemca/pdns-api/internal/app/worker/handler/v1"
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
	config             config.Config
	consul             *api.Client
	logger             *logrus.Logger
	publicHTTPServer   *http.Server
	internalHTTPServer *http.Server
}

func NewApp(cfg config.Config, logger *logrus.Logger) *app {
	logger.Debug("Create new Worker app")

	internalAddr := net.JoinHostPort(cfg.Internal.Address, cfg.Internal.Port)
	return &app{
		config: cfg,
		logger: logger,
		internalHTTPServer: &http.Server{
			Addr: internalAddr,
		},
	}
}

//The entry point of pdns-api
func (a *app) Run(withHealth bool) {
	a.logger.Debug("Run Worker app")

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

	internalRouter := mux.NewRouter()

	flushHandler := workerV1.NewFlushHandler(a.config, prometheusStats, authPowerDNSClient, recursorPowerDNSClient, a.logger)
	internalAddForwardZoneHandler := workerV1.NewAddForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	deleteForwardZoneHandler := workerV1.NewDeleteForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	deleteForwardZonesHandler := workerV1.NewDeleteForwardZonesHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)
	updateForwardZonesHandler := workerV1.NewUpdateForwardZoneHandler(
		a.config,
		prometheusStats,
		authPowerDNSClient,
		recursorPowerDNSClient,
		a.logger,
		errorWriter,
		compositeFZStorage,
	)

	// HTTP internal Handlers
	internalRouter.HandleFunc("/api/v1/internal/{serverID}/cache/flush", flushHandler.FlushInternal).Methods(http.MethodPut)
	internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones", internalAddForwardZoneHandler.AddForwardZonesInternal).Methods(http.MethodPost)
	internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones", deleteForwardZonesHandler.DeleteForwardZonesInternal).Methods(http.MethodDelete)
	internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones/{zoneID}", updateForwardZonesHandler.UpdateForwardZonesInternal).Methods(http.MethodPatch)
	internalRouter.HandleFunc("/api/v1/internal/{serverID}/forward-zones/{zoneID}", deleteForwardZoneHandler.DeleteForwardZoneInternal).Methods(http.MethodDelete)

	a.internalHTTPServer.Handler = internalRouter
	a.internalHTTPServer.TLSConfig = internalService.ServerTLSConfig()
	go func() {
		if err := a.internalHTTPServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			a.logger.WithFields(logrus.Fields{
				"action": log.ActionSystem,
			}).Fatalf("error occurred while running internal http server: %s\n", err.Error())
		}
	}()

	if withHealth {
		a.startPublicServer()
	}

	a.logger.Infof("Internal HTTP server started and listen on %s", net.JoinHostPort(a.config.Internal.Address, a.config.Internal.Port))

}

func (a *app) Shutdown(ctx context.Context, withHealth bool) error {
	if err := a.internalHTTPServer.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		a.logger.Errorf("Stopping internal HTTP server: %v", err)
		return err
	}
	a.logger.Info("Internal HTTP server successfully stopped")

	if withHealth {
		if err := a.publicHTTPServer.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			a.logger.Errorf("Stopping public HTTP server: %v", err)
			return err
		}
	}
	a.logger.Info("Public HTTP server successfully stopped")

	return nil
}

func (a *app) startPublicServer() {
	publicRouter := mux.NewRouter()
	// Prometheus metrics
	publicRouter.Handle("/metrics", promhttp.Handler())

	healthHandler := commonV1.NewHealthHandler(a.config)
	publicRouter.HandleFunc("/api/v1/health", healthHandler.Health).Methods(http.MethodGet)

	publicAddr := net.JoinHostPort(a.config.PublicHTTP.Address, a.config.PublicHTTP.Port)
	a.publicHTTPServer = &http.Server{
		Addr:    publicAddr,
		Handler: publicRouter,
	}
	go func() {
		if err := a.publicHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.WithFields(logrus.Fields{
				"action": log.ActionSystem,
			}).Fatalf("error occurred while running public http server: %s\n", err.Error())
		}
	}()
	a.logger.Infof("Public HTTP server started and listen on %s", publicAddr)
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
