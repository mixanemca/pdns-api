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

package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Role           string       `mapstructure:"role"`
	DataCenter     string       `mapstructure:"datacenter"`
	Environment    string       `mapstructure:"environment"`
	PublicHTTP     HTTPConfig   `mapstructure:"public-http"`
	Log            LogConfig    `mapstructure:"log"`
	BackendTimeout int64        `mapstructure:"backend-timeout"`
	PDNS           PDNSConfig   `mapstructure:"pdns"`
	Consul         ConsulConfig `mapstructure:"consul"`
}

type HTTPConfig struct {
	Address string            `mapstructure:"listen-address"`
	Port    string            `mapstructure:"listen-port"`
	Timeout HTTPTimeoutConfig `mapstructure:"timeout"`
}

type HTTPTimeoutConfig struct {
	Read  int `mapstructure:"read"`
	Write int `mapstructure:"write"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

type PDNSConfig struct {
	BaseURL string `mastructure:"base-url"`
	ApiKey  string `mapstructure:"api-key"`
}

type ConsulConfig struct {
	Address string `mastructure:"address"`
}

func Init() (*Config, error) {
	viper.AddConfigPath("configs")
	viper.AddConfigPath("/etc/pdns-api")
	viper.SetConfigName("pdns-api")
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config

	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
