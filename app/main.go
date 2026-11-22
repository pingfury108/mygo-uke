package main

import (
	"log"

	"mygo-uke/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
