package handlers

import (
	"net/http"
	"time"

	"github.com/SDZZGNDRC/DKV/src/kvraft"
	"github.com/SDZZGNDRC/DKV/src/types"
	"github.com/gin-gonic/gin"
)

func NewHandlers_GetSysStatus(reqChan chan struct{}, respChan chan *types.SysStatus) gin.HandlerFunc {
	handler := func(c *gin.Context) {
		// 清空respChan中的旧数据
	loop:
		for {
			select {
			case <-respChan:
			default:
				break loop
			}
		}

		// 发送状态请求
		select {
		case reqChan <- struct{}{}:
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "ERR_SERVER_BUSY",
				"message": "system is busy, please try again later",
			})
			return
		}

		// 等待响应，设置5秒超时
		select {
		case sysStatus := <-respChan:
			if sysStatus == nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   kvraft.ErrChanClose,
					"message": "system status channel closed",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"role":      sysStatus.Role,
				"server_id": sysStatus.ServerId,
				"term":      sysStatus.Term,
				"voted_for": sysStatus.VotedFor,
				"timestamp": sysStatus.Timestamp,
			})

		case <-time.After(5 * time.Second):
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"error":   kvraft.ErrHandleOpTimeOut,
				"message": "get system status timeout",
			})
		}
	}
	return handler
}
