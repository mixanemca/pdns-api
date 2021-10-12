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
	"github.com/mixanemca/pdns-api/internal/app"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/consul"
	logger2 "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
	"log"
)

const configPath = "config/pdns-api"

func main() {
	log.Println("read config")
	cfg, err := config.Init(configPath)
	if err != nil {
		logrus.Fatalf("error occurred while reading config: %s\n", err.Error())
	}

	if cfg == nil {
		logrus.Errorf("config is empty")
		return
	}

	logger := logger2.NewLogger(cfg.Log.LogFile, cfg.Log.LogLevel)
	consulClient, err := consul.NewConsulClient()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"action": logger2.ActionSystem,
		}).Fatalf("Cannot create a Consul API client: %v", err)
	}

	pdnsApp := app.NewApp(*cfg, consulClient)

	pdnsApp.Run()
}
