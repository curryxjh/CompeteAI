package main

import (
	"CompeteAI/settings"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	configFile := filepath.Join("config", "dev.yaml")
	if len(os.Args) >= 2 && os.Args[1] != "" {
		configFile = os.Args[1]
	}

	if err := settings.Init(configFile); err != nil {
		fmt.Printf("Load config failed, err:%v\n", err)
		return
	}
	
	server := InitWebServer()

	// Define a simple GET endpoint
	server.GET("/ping", func(c *gin.Context) {
		// Return JSON response
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	server.Run(fmt.Sprintf(":%d", settings.Conf.Port))
}
