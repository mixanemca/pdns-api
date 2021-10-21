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

package client

import (
	"fmt"
	"net/http"

	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
)

// FlushAllCache Flush a cache-entry by name for all available services
func (s *client) FlushAllCache(serverID, name string) error {
	// Make an InternalRequest and send it to all alive services
	path := fmt.Sprintf("/api/v1/internal/%s/cache/flush?domain=%s", serverID, name)
	ireq := NewInternalRequest(
		http.MethodPut,
		path,
		nil,
	)
	if err := s.DoInternalRequest(ireq); err != nil {
		return errors.Wrap(err, "flushing caches")
	}

	return nil
}

// AddZone Add a new zone by name for all available services
func (s *client) AddZone(serverID, zoneType string, bodyBytes []byte) error {
	// Make an InternalRequest and send it to all alive services
	path := fmt.Sprintf("/api/v1/internal/%s/%s", serverID, zoneType)
	ireq := NewInternalRequest(
		http.MethodPost,
		path,
		bodyBytes,
	)
	if err := s.DoInternalRequest(ireq); err != nil {
		return errors.Wrap(err, "add zone")
	}

	return nil
}
