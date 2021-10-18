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

	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/infrastructure/consul"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gorilla/mux"
	"github.com/mixanemca/pdns-api/internal/app/config"
	v1 "github.com/mixanemca/pdns-api/internal/app/handler/v1"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

type app struct {
	cfg config.Config
	// consul         *api.Client
	logger         *logrus.Logger
	internalRouter *mux.Router
	publicRouter   *mux.Router
}

func NewApp(cfg config.Config, logger *logrus.Logger) *app {
	publicRouter := mux.NewRouter()
	internalRouter := mux.NewRouter()

	return &app{
		cfg: cfg,
		// consul:         consul,
		logger:         logger,
		internalRouter: internalRouter,
		publicRouter:   publicRouter,
	}
}

//The entry point of pdns-api
func (a *app) Run() {
	powerDNSClient, err := pdnsApi.New(
		pdnsApi.WithBaseURL(a.cfg.PDNS.BaseURL),
		pdnsApi.WithAPIKeyAuthentication(a.cfg.PDNS.ApiKey),
	)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a PowerDNS Authoritative API client: %v", err)
	}

	_, err = consul.NewConsulClient(a.cfg)
	if err != nil {
		a.logger.WithFields(logrus.Fields{
			"action": log.ActionSystem,
		}).Fatalf("Cannot create a Consul API client: %v", err)
	}

	stats := a.initStats()

	healthHandler := v1.NewHealthHandler(a.cfg)
	listServersHandler := v1.NewListServersHandler(a.cfg, stats, powerDNSClient)
	listServerHandler := v1.NewListServerHandler(a.cfg, stats, powerDNSClient)
	searchDataHandler := v1.NewListServerHandler(a.cfg, stats, powerDNSClient)
	forwardZonesHandler := v1.NewForwardZonesHandler(a.cfg, stats, powerDNSClient)

	// HTTP Handlers
	a.publicRouter.HandleFunc("/api/v1/health", healthHandler.Health).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers", listServersHandler.ListServers).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}", listServerHandler.ListServer).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/search-data", searchDataHandler.SearchData).Methods(http.MethodGet)
	a.publicRouter.HandleFunc("/api/v1/servers/{serverID}/forward-zones", forwardZonesHandler.ListForwardZones).Methods(http.MethodGet)

	// HTTP Server
	publicAddr := net.JoinHostPort(a.cfg.PublicHTTP.Address, a.cfg.PublicHTTP.Port)

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

	a.logger.Infof("Server started and listen on %s", net.JoinHostPort(a.cfg.PublicHTTP.Address, a.cfg.PublicHTTP.Port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	<-quit

	a.logger.Info("Server stopped")
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
