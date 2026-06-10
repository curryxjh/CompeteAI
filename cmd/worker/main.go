package main

import (
	"CompeteAI/internal/app"
	"log"
)

func main() {
	if err := app.RunWorker(); err != nil {
		log.Fatal(err)
	}
}
