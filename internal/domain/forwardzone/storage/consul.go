package storage

import (
	"encoding/json"
	"github.com/hashicorp/consul/api"
	"github.com/mixanemca/pdns-api/internal/domain/forwardzone"
	"github.com/mixanemca/pdns-api/internal/infrastructure/errors"
)

type ConsuleStorage struct {
	consul *api.Client
}

func NewConsuleStorage(consul *api.Client) *ConsuleStorage {
	return &ConsuleStorage{consul: consul}
}

func (s *ConsuleStorage) Save(fzs []forwardzone.ForwardZone) error {
	kv := s.consul.KV()
	value, err := json.Marshal(fzs)
	if err != nil {
		return errors.Wrap(err, "writing forward-zones to Consul")
	}
	p := &api.KVPair{Key: forwardzone.ForwardZonesConsulKVKey, Value: value}
	_, err = kv.Put(p, nil)
	if err != nil {
		return errors.Wrap(err, "writing forward-zones to Consul")
	}
	return nil
}