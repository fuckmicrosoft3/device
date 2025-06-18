// services/device/main.go
package main

import (
	"log"
	"os"

	"go.novek.io/device/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
