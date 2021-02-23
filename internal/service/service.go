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

package service

import (
	"github.com/mixanemca/pdns-api/internal/models"
	"github.com/mixanemca/pdns-api/internal/repository"
)

//go:generate mockgen -source=service.go -destination=mocks/mock.go

type PDNS interface {
	Version() (*models.Version, error)
}

type Service struct {
	PDNS
}

func NewService(repos *repository.Repository) *Service {
	return &Service{
		// PDNS: NewPDNSService(repos.PDNS),
	}
}
