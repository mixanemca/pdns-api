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
	Role        string       `mapstructure:"role"`
	DataCenter  string       `mapstructure:"datacenter"`
	Environment string       `mapstructure:"environment"`
	PublicHTTP  HTTPConfig   `mapstructure:"public-http"`
	Log         LogConfig    `mapstructure:"log"`
	PDNS        PDNSConfig   `mapstructure:"pdns"`
	Consul      ConsulConfig `mapstructure:"consul"`
	Version     string
	Build       string
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
	BaseURL string `mapstructure:"base-url"`
	ApiKey  string `mapstructure:"api-key"`
	Timeout int    `mapstructure:"timeout"`
}

type ConsulConfig struct {
	Address string `mastructure:"address"`
}

func Init(version, build string) (*Config, error) {
	viper.AddConfigPath("configs")
	viper.AddConfigPath("/etc/pdns-api")
	viper.SetConfigName("pdns-api")
	viper.SetConfigType("yaml")

	// Set configuration defaults
	viper.SetDefault("datacenter", "dataspace")
	viper.SetDefault("environment", "dev")
	viper.SetDefault("public-http.listen-address", "127.0.0.1")
	viper.SetDefault("public-http.listen-port", 8080)
	viper.SetDefault("pdns.base-url", "http://127.0.0.1:8081")
	viper.SetDefault("pdns.timeout", 10)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config

	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	cfg.Version = version
	cfg.Build = build

	return &cfg, nil
}
