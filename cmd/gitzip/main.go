package main

import (
	"os"

	"gitzip/internal/app"
)

func main() {
	if err := app.Run(os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}
