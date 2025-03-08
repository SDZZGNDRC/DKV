package utils

import (
	"log"
	"net/http"

	"github.com/SDZZGNDRC/DKV/src/config"
	"github.com/gin-gonic/gin"
)

func TokenValid(c *gin.Context) bool {
	tokenString := c.GetHeader("Token")
	for _, i := range config.Config.APIAuthTokens {
		if tokenString == i {
			return true
		}
	}
	log.Println("Token", tokenString, "is not valid; should be one of", config.Config.APIAuthTokens)
	return false
}

func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !TokenValid(c) {
			c.String(http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}
