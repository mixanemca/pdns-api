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
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mixanemca/pdns-api/internal/config"
	"github.com/sirupsen/logrus"
)

var (
	version string = "unknown"
	build   string = "unknown"
)

func Run(configPath string) {
	log.Println("read config")
	cfg, err := config.Init(configPath)
	if err != nil {
		// logger.Error(err)
		logrus.Fatalf("error occurred while reading config: %s\n", err.Error())
		return
	}

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

	logrus.Infof("Server started and listen on %s:%s", cfg.HTTP.Address, cfg.HTTP.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	<-quit

	logrus.Info("Server stopped")
}
