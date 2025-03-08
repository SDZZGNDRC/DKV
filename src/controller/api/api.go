package api

import (
	"net/http"

	"github.com/SDZZGNDRC/DKV/src/controller/api/handlers"
	"github.com/SDZZGNDRC/DKV/src/controller/api/utils"
	"github.com/SDZZGNDRC/DKV/src/types"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE, PUT")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")

		//放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		// 处理请求
		c.Next()
	}
}

func InitAPI(
	host string,
	apiChans types.APIChans,
) {
	r := gin.Default()
	r.Use(CORSMiddleware())

	api := r.Group("/api")
	api.Use(utils.TokenAuthMiddleware()) // 再应用认证

	// 路由注册
	api.GET("/get-sysstatus", handlers.NewHandlers_GetSysStatus(apiChans.GetSysStatusReqChan, apiChans.GetSysStatusRespChan))

	go r.Run(host)
}
