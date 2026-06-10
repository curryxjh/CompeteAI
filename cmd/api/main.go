package main

import (
	"CompeteAI/internal/app"
	"log"
	"os"
	"strings"
)

var apiSubcommands = map[string]bool{
	"server": true, "api": true, "run": true, "start": true,
}

func main() {
	configFile := app.ConfigFromEnv()
	if len(os.Args) >= 2 {
		arg := os.Args[1]
		if !apiSubcommands[strings.ToLower(arg)] &&
			(strings.Contains(arg, "/") || strings.Contains(arg, ".yaml") ||
				strings.Contains(arg, ".yml") || strings.Contains(arg, ".json")) {
			configFile = arg
		}
	}
	if err := app.RunAPI(configFile); err != nil {
		log.Fatal(err)
	}
}
