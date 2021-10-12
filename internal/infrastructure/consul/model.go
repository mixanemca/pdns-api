package consul

import "github.com/mixanemca/pdns-api/internal/infrastructure"

type consuleAgent struct {
	Name     string
	Addres   string
	ID       string
	Port     int
	Url      string
	Interval string
	Timeout  string
	IsNative bool
	Header   map[string][]string
}

// Todo Move it to config
var consuleAgentsForInteralService = []consuleAgent{
	{
		Name:     pdnsServiceName,
		Addres:   infrastructure.GetHostname(),
		ID:       pdnsServiceName,
		Port:     8080,
		Url:      "http://127.0.0.1:8080/api/v1/health",
		Interval: "2s",
		Timeout:  "1s",
		IsNative: true,
	},
	{
		Name:     pdnsInternalServiceName,
		Addres:   infrastructure.GetHostname(),
		ID:       pdnsInternalServiceName,
		Port:     8090,
		Url:      "http://127.0.0.1:8080/api/v1/health",
		Interval: "2s",
		Timeout:  "1s",
		IsNative: true,
	},
	{
		Name:     pdnsAuthoritativeServiceName,
		Addres:   infrastructure.GetHostname(),
		ID:       pdnsAuthoritativeServiceName,
		Port:     8081,
		Url:      "http://127.0.0.1:8081/api/v1/servers",
		Interval: "2s",
		Timeout:  "1s",
		Header:   map[string][]string{"X-API-Key": {"pdns"}},
	},
	{
		Name:     pdnsRecursorServiceName,
		Addres:   infrastructure.GetHostname(),
		ID:       pdnsRecursorServiceName,
		Port:     8082,
		Url:      "http://127.0.0.1:8082/api/v1/servers",
		Interval: "2s",
		Timeout:  "1s",
		Header:   map[string][]string{"X-API-Key": {"pdns"}},
	},
}
