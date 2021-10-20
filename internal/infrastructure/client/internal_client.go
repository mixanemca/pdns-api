package client

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	agConnect "github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/mixanemca/pdns-api/internal/app/config"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"golang.org/x/sync/errgroup"
)

const (
	PDNSServiceName         string = "pdns-api"
	PDNSInternalServiceName string = "pdns-api-internal"
	consulDC                string = "dc1"
	consulNamespace         string = "default"
)

// InternalRequest holds params for do requests via internal API
type InternalRequest struct {
	method string
	path   string
	data   []byte
	// data   io.Reader
}

type connDialer struct {
	c net.Conn
}

func (cd connDialer) Dial(network, addr string) (net.Conn, error) {
	return cd.c, nil
}

// NewInternalRequest creates a new InternalRequest
func NewInternalRequest(method, path string, data []byte) *InternalRequest {
	return &InternalRequest{
		method: method,
		path:   path,
		data:   data,
	}
}

// todo refactor it
type client struct {
	config          config.Config
	consulClient    *api.Client
	internalService *connect.Service
}

func NewClient(config config.Config, consulClient *api.Client, internalService *connect.Service) *client {
	return &client{config: config, consulClient: consulClient, internalService: internalService}
}

// InternalRequest do requests via internal API to healthy services.
func (s *client) DoInternalRequest(ireq *InternalRequest) error {
	// Get healthy service entries fom Consul
	serviceEntries, _, err := s.consulClient.Health().Service(PDNSServiceName, "", true, &api.QueryOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get healthy service %s entries from Consul", PDNSServiceName)
	}

	// Creating an errgroup.Group for doing requests into internal API
	g := new(errgroup.Group)

	for _, entry := range serviceEntries {
		// https://golang.org/doc/faq#closures_and_goroutines
		addr := entry.Service.Address
		port := s.config.Internal.ListenPort
		p := ireq.path
		var buf bytes.Buffer
		if ireq.data != nil {
			_, _ = io.Copy(&buf, ioutil.NopCloser(bytes.NewReader(ireq.data)))
		}

		g.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.BackendTimeout)*time.Second)
			defer cancel()

			// https://www.consul.io/docs/connect/native/go
			// connect.HTTPClient() internally do resolve single node by service or query,
			// and do request to only one this node.
			// Instead of this we use raw TLS Connection.
			// todo move to config
			conn, _ := s.internalService.Dial(ctx, &connect.StaticResolver{
				Addr: fmt.Sprintf("%s:%d", addr, port),
				CertURI: &agConnect.SpiffeIDService{
					Namespace:  consulNamespace,
					Datacenter: consulDC,
					Service:    PDNSInternalServiceName,
				},
			})
			defer conn.Close()

			t := &http.Transport{
				DialTLS: connDialer{conn}.Dial,
			}
			// Configures a net/http HTTP/1 Transport to use HTTP/2.
			_ = http2.ConfigureTransport(t)

			httpClient := &http.Client{
				Transport: t,
				Timeout:   time.Duration(s.config.BackendTimeout) * time.Second,
			}
			url := fmt.Sprintf("https://%s:%d%s", addr, port, p)

			req, err := http.NewRequest(ireq.method, url, &buf)
			if err != nil {
				return err
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()

			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return errors.Wrap(err, "failed to do requests via internal API")
	}

	return nil
}
