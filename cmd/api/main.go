package main

import (
	"CompeteAI/internal/app"
	"log"
	"os"
)

func main() {
	configFile := app.ConfigFromEnv()
	if len(os.Args) >= 2 && os.Args[1] != "" {
		configFile = os.Args[1]
	}
	if err := app.RunAPI(configFile); err != nil {
		log.Fatal(err)
	}
}
