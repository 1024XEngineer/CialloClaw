package main

import (
	"log"
	"os"
	"time"

	"cialloclaw.local/backend/internal/data"
	"cialloclaw.local/backend/internal/server"
	"cialloclaw.local/backend/internal/store"
)

func main() {
	addr := os.Getenv("CIALLO_CLAW_ADDR")
	if addr == "" {
		addr = "127.0.0.1:17888"
	}

	mock := data.Build(time.Now())
	st := store.New(mock)
	srv := server.New(addr, st)

	log.Printf("CialloClaw mock backend listening on http://%s", addr)
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
