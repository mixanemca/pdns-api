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

package v1

import (
	"encoding/json"
	"net/http"

	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure"
)

type alive struct {
	Status   bool   `json:"alive"`
	Hostname string `json:"hostname"`
}

type HealthHandler struct {
	config config.Config
}

func NewHealthHandler(c config.Config) *HealthHandler {
	return &HealthHandler{config: c}
}

// Health return json with alive status
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	// timer := getLabeledResponseTimePeersHistogramTimer(s.Config.Environment, s.Hostname, r.URL.Path, r.Method)
	// defer timer.ObserveDuration()

	// TODO: add real checks
	a := alive{
		Status:   true,
		Hostname: infrastructure.GetHostname(),
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		// countError(h.config.Environment, infrastructure.GetHostname(), r.URL.Path, http.StatusInternalServerError)
	}
	// countCall(h.config.Environment, infrastucture.GetHostname(), r.URL.Path, r.Method, http.StatusOK)
}
