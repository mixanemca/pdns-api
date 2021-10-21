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
