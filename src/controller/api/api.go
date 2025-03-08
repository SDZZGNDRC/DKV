package api

import (
	"github.com/SDZZGNDRC/DKV/src/controller/api/handlers"
	"github.com/SDZZGNDRC/DKV/src/controller/api/utils"
	"github.com/SDZZGNDRC/DKV/src/types"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Token, Content-Type")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 预检请求缓存1天

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func InitAPI(
	host string,
	apiChans types.APIChans,
) {
	r := gin.Default()

	// 在路由组中添加CORS中间件（注意顺序）
	api := r.Group("/api")
	api.Use(CORSMiddleware())            // 先应用CORS
	api.Use(utils.TokenAuthMiddleware()) // 再应用认证

	// 路由注册
	api.GET("/get-sysstatus", handlers.NewHandlers_GetSysStatus(apiChans.GetSysStatusReqChan, apiChans.GetSysStatusRespChan))

	go r.Run(host)
}
