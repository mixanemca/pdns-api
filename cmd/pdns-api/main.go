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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	if cfg.Role == config.ROLE_WORKER {
		withHealth := true
		workerApp := worker.NewApp(*cfg, logger)
		workerApp.Run(withHealth)

		<-quit

		ctxInternal, cancelInternal := context.WithTimeout(context.Background(), time.Duration(cfg.Internal.Timeout.Read)*time.Second)
		defer cancelInternal()

		err := workerApp.Shutdown(ctxInternal, withHealth)
		if err != nil {
			logger.Error(err)
		}
	} else {
		withHealth := false
		workerApp := worker.NewApp(*cfg, logger)
		workerApp.Run(withHealth)
		apiApp := api.NewApp(*cfg, logger)
		apiApp.Run()

		<-quit

		ctxWorker, cancelWorker := context.WithTimeout(context.Background(), time.Duration(cfg.Internal.Timeout.Read)*time.Second)
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
