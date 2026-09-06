package main

import (
	"smallgo/server/config"
	"smallgo/server/server"

	// Blank-import apps so their init() registers routes with the app registry.
	// Add your own apps here.
	_ "smallgo/server/billing"
	_ "smallgo/server/qrcode"
)

func main() {
	config.Parse()
	server.Start()
}
