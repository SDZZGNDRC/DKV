package api

import (
	"github.com/SDZZGNDRC/DKV/src/controller/api/utils"
	"github.com/gin-gonic/gin"
)

func InitAPI(
	host string,
	apiChans APIChans,
) {
	r := gin.Default()

	api := r.Group("/api")
	api.Use(utils.TokenAuthMiddleware())

	// 注册路由

	r.Run(host)
}
