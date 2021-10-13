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

	// pdnshttpClient := pdnshttp.NewClient(cfg.PDNSHTTP.Key)

	// services := service.NewServices(pdnshttpClient)

	// handlers := httpv1.NewHandler(services.PDNSHTTP)

	// HTTP Handlers
	a.publicRouter.HandleFunc("/api/v1/health", v1.Health).Methods(http.MethodGet)
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
