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
	HTTP     HTTPConfig `mapstructure:"http"`
	PDNSHTTP PDNSHTTPConfig
	Log      LogConfig
}

type HTTPConfig struct {
	Address string `mapstructure:"address"`
	Port    string `mapstructure:"port"`
	Timeout HTTPTimeoutConfig
}

type HTTPTimeoutConfig struct {
	Read  int
	Write int
}

type PDNSHTTPConfig struct {
	Key string `mapstructure:"pdnshhtp.key"`
}

type LogConfig struct {
	LogLevel string `mapstructure:"log-level"`
	LogFile  string `mapstructure:"log-file"`
}

func Init(configPath string) (*Config, error) {
	viper.AddConfigPath("configs")
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
