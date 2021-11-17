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

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mixanemca/pdns-api/internal/app/api"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/app/worker"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	version string = "unknown"
	build   string = "unknown"
)

func main() {
	cfg, err := config.Init(version, build)
	if err != nil {
		logrus.Fatalf("error occurred while reading config: %s\n", err.Error())
	}

	if cfg == nil {
		logrus.Errorf("config is empty")
		return
	}

	logger := log.NewLogger(cfg.Log.File, cfg.Log.Level)

	logger.Infof("Version: %s; Build: %s", cfg.Version, cfg.Build)
	logger.Infof("Server start as a role %s", cfg.Role)

	stats := initStats()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	if cfg.Role == config.ROLE_WORKER {
		withHealth := true
		workerApp := worker.NewApp(*cfg, logger)
		workerApp.Run(stats, withHealth)

		<-quit

		ctxInternal, cancelInternal := context.WithTimeout(context.Background(), time.Duration(cfg.InternalHTTP.Timeout.Read)*time.Second)
		defer cancelInternal()

		err := workerApp.Shutdown(ctxInternal, withHealth)
		if err != nil {
			logger.Error(err)
		}
	} else {
		withHealth := false
		workerApp := worker.NewApp(*cfg, logger)
		workerApp.Run(stats, withHealth)
		apiApp := api.NewApp(*cfg, logger)
		apiApp.Run(stats)

		<-quit

		ctxWorker, cancelWorker := context.WithTimeout(context.Background(), time.Duration(cfg.InternalHTTP.Timeout.Read)*time.Second)
		defer cancelWorker()
		err := workerApp.Shutdown(ctxWorker, withHealth)
		if err != nil {
			logger.Error(err)
		}

		ctxApi, cancelApi := context.WithTimeout(context.Background(), time.Duration(cfg.PublicHTTP.Timeout.Read)*time.Second)
		defer cancelApi()
		err = apiApp.Shutdown(ctxApi)
		if err != nil {
			logger.Error(err)
		}
	}

	logger.Info("Server successfully stopped")
}

func initStats() *stats.PrometheusStats {
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
