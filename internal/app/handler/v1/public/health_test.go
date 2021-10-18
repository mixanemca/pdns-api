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

package public

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	conf := config.Config{}
	s := NewHealthHandler(conf)

	req, err := http.NewRequest("GET", "/api/v1/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	s.Health(rr, req)

	require.Equal(t, rr.Code, http.StatusOK, fmt.Sprintf("handler returned wrong status code: got %v want %v",
		rr.Code, http.StatusOK))
	require.Equal(
		t,
		rr.Header().Get("Content-Type"),
		"application/json;charset=utf-8", fmt.Sprintf("content type header does not match: got %v want %v",
			rr.Header().Get("Content-Type"), "application/json"))

	var a alive
	err = json.Unmarshal(rr.Body.Bytes(), &a)
	require.NoError(t, err)
}
