package main

import (
	"CompeteAI/internal/app"
	"log"
)

func main() {
	if err := app.RunRecovery(); err != nil {
		log.Fatal(err)
	}
}
