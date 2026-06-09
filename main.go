package main

import (
	"CompeteAI/internal/app"
	"log"
	"os"
)

// 根目录 main 为 API 进程（兼容旧启动方式）。Worker: go run ./cmd/worker
func main() {
	configFile := app.ConfigFromEnv()
	if len(os.Args) >= 2 && os.Args[1] != "" {
		configFile = os.Args[1]
	}
	if err := app.RunAPI(configFile); err != nil {
		log.Fatal(err)
	}
}
