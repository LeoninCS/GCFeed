package router

import (
	"github.com/gin-gonic/gin"
)

func Register(g *gin.Engine) error {
	g.GET("/health", HealthCheck)
	return nil
}

func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "All is well",
	})
}
