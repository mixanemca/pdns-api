package pdns

const (
	zoneTypeZone        = "zones"
	zoneTypeForwardZone = "forward-zones"
)

const (
	cnTypeReplace = "replace"
	cnTypeDelete  = "delete"
)

// TODO: move to config
const (
	forwardZonesFile        = "/etc/powerdns/forward-zones.conf"
	forwardZonesConsulKVKey = "forward-zones"
)

const (
	defaultMaxResults string = "10"
)

const (
	localNameserver  string = "127.0.0.1:5353"
	localReverseZone string = "10.in-addr.arpa"
)
