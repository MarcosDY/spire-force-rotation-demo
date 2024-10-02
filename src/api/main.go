package main

import (
	"api/pkg/service"
	"os"
)

func main() {
	if err := service.Start(); err != nil {
		os.Exit(1)
	}
}
