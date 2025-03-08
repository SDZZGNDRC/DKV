package utils

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var Authenticated_Tokens = viper.GetStringSlice("API-Authenticated-Tokens")

func TokenValid(c *gin.Context) bool {
	tokenString := c.GetHeader("Token")
	for _, i := range Authenticated_Tokens {
		if tokenString == i {
			return true
		}
	}
	log.Println("Token", tokenString, "is not valid; should be one of", Authenticated_Tokens)
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
