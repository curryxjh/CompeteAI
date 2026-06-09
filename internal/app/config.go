package app

import (
	"CompeteAI/settings"
	"fmt"
	"os"
	"path/filepath"
)

func LoadConfig() error {
	configFile := filepath.Join("config", "dev.yaml")
	if len(os.Args) >= 2 && os.Args[1] != "" && os.Args[1][0] != '-' {
		configFile = os.Args[1]
	}
	if err := settings.Init(configFile); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return settings.InitLogger()
}

func WorkerRole() string {
	if r := os.Getenv("WORKER_ROLE"); r != "" {
		return r
	}
	return "all"
}

func WorkerID() string {
	if id := os.Getenv("WORKER_ID"); id != "" {
		return id
	}
	host, _ := os.Hostname()
	if host == "" {
		return "worker-local"
	}
	return host
}
