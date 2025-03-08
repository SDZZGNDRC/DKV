package api

import (
	"github.com/SDZZGNDRC/DKV/src/controller/api/handlers"
	"github.com/SDZZGNDRC/DKV/src/controller/api/utils"
	"github.com/SDZZGNDRC/DKV/src/types"
	"github.com/gin-gonic/gin"
)

func InitAPI(
	host string,
	apiChans types.APIChans,
) {
	r := gin.Default()

	api := r.Group("/api")
	api.Use(utils.TokenAuthMiddleware())

	// 注册路由
	api.GET("/get-sysstatus", handlers.NewHandlers_GetSysStatus(apiChans.GetSysStatusReqChan, apiChans.GetSysStatusRespChan))

	go r.Run(host)
}
