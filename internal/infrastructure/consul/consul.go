package consul

import (
	"github.com/hashicorp/consul/api"
)

func NewConsulClient() (*api.Client, error) {
	consul, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	err = registerConsuleAgents(consul)
	if err != nil {
		return nil, err
	}

	return consul, nil
}

func registerConsuleAgents(consule *api.Client) error {
	for _, agent := range consuleAgentsForInteralService {
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

		err := consule.Agent().ServiceRegister(serviceRegistration)
		if err != nil {
			return err
		}
	}

	return nil
}