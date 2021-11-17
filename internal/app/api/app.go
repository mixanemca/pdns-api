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

package api

import (
	"context"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	apiV1 "github.com/mixanemca/pdns-api/internal/app/api/handler/v1"
	commonV1 "github.com/mixanemca/pdns-api/internal/app/common/handler/v1"
	"github.com/mixanemca/pdns-api/internal/app/middleware"
	"github.com/mixanemca/pdns-api/internal/domain/zone"
	"github.com/mixanemca/pdns-api/internal/infrastructure/client"
	"github.com/mixanemca/pdns-api/internal/infrastructure/ldap"
	"github.com/mixanemca/pdns-api/internal/infrastructure/network"
	"github.com/spf13/viper"

	pdnsApi "github.com/mittwald/go-powerdns"
	"github.com/mixanemca/pdns-api/internal/infrastructure/consul"
	"github.com/mixanemca/pdns-api/internal/infrastructure/stats"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mixanemca/pdns-api/internal/app/config"
	log "github.com/mixanemca/pdns-api/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

type app struct {
	config           config.Config
	consul           *api.Client
	logger           *logrus.Logger
	publicHTTPServer *http.Server
}

func NewApp(cfg config.Config, logger *logrus.Logger) *app {
	logger.Debug("Create new API app")

	publicAddr := net.JoinHostPort(cfg.PublicHTTP.Address, cfg.PublicHTTP.Port)
	return &app{
		config: cfg,
		logger: logger,
		publicHTTPServer: &http.Server{
			Addr: publicAddr,
		},
	}
}

//The entry point of pdns-api
func (a *app) Run(prometheusStats *stats.PrometheusStats) {
	a.logger.Debug("Run API app")

	authPowerDNSClient, err := pdnsApi.New(
		pdnsApi.WithBaseURL(a.config.PDNS.AuthConfig.BaseURL),
		pdnsApi.WithAPIKeyAuthentication(a.config.PDNS.AuthConfig.ApiKey),
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

	// prometheusStats := a.initStats()

	errorWriter := network.NewErrorWriter(a.config, a.logger, prometheusStats)

	healthHandler := commonV1.NewHealthHandler(a.config)
	listServersHandler := apiV1.NewListServersHandler(a.config, errorWriter, prometheusStats, a.logger, authPowerDNSClient)
	listServerHandler := apiV1.NewListServerHandler(a.config, prometheusStats, authPowerDNSClient)
	searchDataHandler := apiV1.NewListServerHandler(a.config, prometheusStats, authPowerDNSClient)
	forwardZonesHandler := apiV1.NewForwardZonesHandler(a.config, prometheusStats, authPowerDNSClient)
	zonesHandler := apiV1.NewZonesHandler(a.config, prometheusStats, authPowerDNSClient)
	versionHandler := apiV1.NewVersionHandler(a.config, prometheusStats)

	publicRouter := mux.NewRouter()
	// HTTP public Handlers
	publicRouter.HandleFunc("/api/v1/health", healthHandler.Health).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/servers", listServersHandler.ListServers).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/servers/{serverID}", listServerHandler.ListServer).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/servers/{serverID}/search-data", searchDataHandler.SearchData).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/servers/{serverID}/forward-zones", forwardZonesHandler.ListForwardZones).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/servers/{serverID}/forward-zones/{zoneID}", forwardZonesHandler.ListForwardZone).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/servers/{serverID}/zones", zonesHandler.ListZones).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/servers/{serverID}/zones/{zoneID}", zonesHandler.ListZone).Methods(http.MethodGet)
	publicRouter.HandleFunc("/api/v1/version", versionHandler.Get).Methods(http.MethodGet)

	// Prometheus metrics
	publicRouter.Handle("/metrics", promhttp.Handler())

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

	addZoneHanler := apiV1.NewAddZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
	)

	deleteZoneHanler := apiV1.NewDeleteZone(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
	)

	patchZoneHanler := apiV1.NewPatchZone(
		a.config,
		errorWriter,
		prometheusStats,
		a.logger,
		authPowerDNSClient,
		ptrRecorder,
		internalClient,
	)
	publicAddForwardZonesHandler := apiV1.NewAddForwardZonesHandler(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		internalClient,
	)
	publicDelForwardZonesHandler := apiV1.NewDelForwardZonesHandler(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		internalClient,
	)
	publicDelForwardZoneHandler := apiV1.NewDelForwardZoneHandler(
		a.config,
		ldapService,
		errorWriter,
		prometheusStats,
		a.logger,
		internalClient,
	)
	publicPatchForwardZoneHandler := apiV1.NewPatchForwardZoneHandler(
		a.config,
		errorWriter,
		prometheusStats,
		internalClient,
	)

	if viper.GetBool("ldap.enabled") {
		authRouter := publicRouter.Methods(http.MethodDelete, http.MethodPatch, http.MethodPost).Subrouter()
		authRouter.Use(authMiddleware.AuthMiddleware)
		// HTTP Handlers with Authorization
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicAddForwardZonesHandler.AddForwardZones).Methods(http.MethodPost)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicDelForwardZonesHandler.DelForwardZones).Methods(http.MethodDelete)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicPatchForwardZoneHandler.PatchForwardZone).Methods(http.MethodPatch)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicDelForwardZoneHandler.DelForwardZone).Methods(http.MethodDelete)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}", addZoneHanler.AddZone).Methods(http.MethodPost)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", patchZoneHanler.PatchZone).Methods(http.MethodPatch)
		authRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", deleteZoneHanler.DeleteZone).Methods(http.MethodDelete)
	} else {
		publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicAddForwardZonesHandler.AddForwardZones).Methods(http.MethodPost)
		publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}", publicDelForwardZonesHandler.DelForwardZones).Methods(http.MethodDelete)
		publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicPatchForwardZoneHandler.PatchForwardZone).Methods(http.MethodPatch)
		publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:forward-zones}/{zoneID}", publicDelForwardZoneHandler.DelForwardZone).Methods(http.MethodDelete)
		publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}", addZoneHanler.AddZone).Methods(http.MethodPost)
		publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", patchZoneHanler.PatchZone).Methods(http.MethodPatch)
		publicRouter.HandleFunc("/api/v1/servers/{serverID}/{zoneType:zones}/{zoneID}", deleteZoneHanler.DeleteZone).Methods(http.MethodDelete)
	}

	a.publicHTTPServer.Handler = publicRouter

	go func() {
		if err := a.publicHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.WithFields(logrus.Fields{
				"action": log.ActionSystem,
			}).Fatalf("error occurred while running http server: %s\n", err.Error())
		}
	}()

	a.logger.Infof("Public HTTP server started and listen on %s", net.JoinHostPort(a.config.PublicHTTP.Address, a.config.PublicHTTP.Port))
}

// Shutdown Shutdown gracefully shuts down the server without interrupting any active connections.
func (a *app) Shutdown(ctx context.Context) error {
	// TODO: Close Consul Connect service for internal API
	if err := consul.ShutdownConsulClinet(a.consul); err != nil {
		a.logger.Errorf("Stopping consul client: %v", err)
		return err
	}
	a.logger.Debug("Consul client successfylly stopped")

	if err := a.publicHTTPServer.Shutdown(ctx); err != nil {
		a.logger.Errorf("Stopping public HTTP server: %v", err)
		return err
	}
	a.logger.Info("Public HTTP server successfully stopped")

	return nil
}
