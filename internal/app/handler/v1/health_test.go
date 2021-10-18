package v1

import (
	"encoding/json"
	"fmt"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
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
