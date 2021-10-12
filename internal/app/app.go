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
	"github.com/hashicorp/consul/api"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

type app struct {
	cfg    config.Config
	consul *api.Client
}

func NewApp(cfg config.Config, consul *api.Client) *app {
	return &app{cfg: cfg, consul: consul}
}

//The entry point of pdns-api
func (a *app) Run() {

	// pdnshttpClient := pdnshttp.NewClient(cfg.PDNSHTTP.Key)

	// services := service.NewServices(pdnshttpClient)

	// handlers := httpv1.NewHandler(services.PDNSHTTP)

	/*
		// HTTP Server
		srv := server.NewServer(cfg, handlers.Init())
		go func() {
			if err := srv.Run(); err != nil {
				logrus.Errorf("error occurred while running http server: %s\n", err.Error())
			}
		}()
	*/

	logrus.Infof("Server started and listen on %s:%s", a.cfg.HTTP.Address, a.cfg.HTTP.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	<-quit

	logrus.Info("Server stopped")
}
