package main

import (
	"smallgo/server/config"
	"smallgo/server/server"

	// Blank-import apps so their init() registers routes with the app registry.
	// Add your own apps here.
	_ "smallgo/server/reminder"
)

func main() {
	// `make dev` starts the binary from the repository root. Loading .env here
	// makes provider credentials available locally without changing production
	// environment-variable precedence.
	config.LoadDotEnv(".env", "server/.env")
	config.Parse()
	server.Start()
}
