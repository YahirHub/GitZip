package main

import (
	"os"

	"gitzip/internal/app"
)

func main() {
	if err := app.Run(os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
