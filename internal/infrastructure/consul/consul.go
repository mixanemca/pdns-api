package consul

import (
	"github.com/hashicorp/consul/api"
	"github.com/mixanemca/pdns-api/internal/app/config"
)

func NewConsulClient(cfg config.Config) (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = cfg.Consul.Address
	consul, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	err = registerConsulAgents(consul)
	if err != nil {
		return nil, err
	}

	return consul, nil
}

func ShutdownConsulClinet(consul *api.Client) error {
	err := deregisterConsulAgents(consul)
	if err != nil {
		return err
	}

	return nil
}

func registerConsulAgents(consul *api.Client) error {
	for _, agent := range consulAgentsForInteralService {
		serviceRegistration := &api.AgentServiceRegistration{
			Name:    agent.Name,
			Address: agent.Addres,
			ID:      agent.ID,
			Port:    agent.Port,
			Check: &api.AgentServiceCheck{
				HTTP:     agent.Url,
				Interval: agent.Interval,
				Timeout:  agent.Timeout,
			},
		}

		if agent.IsNative {
			serviceRegistration.Connect = &api.AgentServiceConnect{
				Native: agent.IsNative,
			}
		}

		if len(agent.Header) > 0 {
			serviceRegistration.Check.Header = agent.Header
		}

		err := consul.Agent().ServiceRegister(serviceRegistration)
		if err != nil {
			return err
		}
	}

	return nil
}

func deregisterConsulAgents(consul *api.Client) error {
	for _, agent := range consulAgentsForInteralService {
		err := consul.Agent().ServiceDeregister(agent.Name)
		if err != nil {
			return err
		}
	}

	return nil
}
