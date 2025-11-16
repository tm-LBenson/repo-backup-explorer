package main

import (
	"log"
	"net/http"

	"repo-backup-explorer/internal/handlers"
)

func main() {
	const addr = "127.0.0.1:9009"

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	log.Printf("Open http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
