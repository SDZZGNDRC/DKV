package handlers

import (
	"net/http"
	"time"

	"github.com/SDZZGNDRC/DKV/src/types"
	"github.com/gin-gonic/gin"
)

func NewHandlers_OpPut(reqChan chan *types.OpPutReq, respChan chan *types.OpPutResp) gin.HandlerFunc {
	handler := func(c *gin.Context) {
	loop:
		for {
			select {
			case <-respChan:
			default:
				break loop
			}
		}
		// body is like {"key": "key", "value": "value"}
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		key, ok := body["key"]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
			return
		}
		value, ok := body["value"]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "value is required"})
			return
		}
		req := &types.OpPutReq{Key: key, Value: value}
		select {
		case reqChan <- req:
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ERR_SERVER_BUSY", "message": "system is busy, please try again later"})
			return
		}
		// 等待响应, 设置5秒超时
		select {
		case resp := <-respChan:
			c.JSON(http.StatusOK, gin.H{"success": resp.Success})
		case <-time.After(5 * time.Second):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ERR_SERVER_BUSY", "message": "system is busy, please try again later"})
			return
		}
	}
	return handler
}
