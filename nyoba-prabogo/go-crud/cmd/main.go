package main

import (
	"embed"
	"os"

	"prabogo/internal"
)

var embedMigrations embed.FS

func main() {
	app := internal.NewApp()
	option := "http"
	if len(os.Args) > 1 {
		option = os.Args[1]
	}

	app.Run(option)
}
