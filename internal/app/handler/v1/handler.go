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

package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/mixanemca/pdns-api/internal/service"
)

type Handler struct {
	// pdnshttpService service.Pdnshttp
	services *pdns.Service
}

// pdnshttpService arg
func NewHandler(services *pdns.Service) *Handler {
	return &Handler{services: services}
}

func (h *Handler) Init() (api *gin.Engine) {
	/*
		v1 := api.Group("/v1")
		{
			// h.initRouter(v1)
		}
	*/
	router := gin.New()

	return router
}