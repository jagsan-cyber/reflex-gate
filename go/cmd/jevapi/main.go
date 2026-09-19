package main

import (
	"log"
	"os"

	"local-jev/internal/api"
	"local-jev/internal/config"
)

func main() {
	cfg := config.Load()

	addr := os.Getenv("JEV_PORT")
	if addr == "" {
		addr = "8090"
	}
	host := os.Getenv("JEV_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	root := os.Getenv("JEV_LLM_BASE_URL")
	if root == "" {
		root = "http://127.0.0.1:8080"
	}
	s := api.NewServer(root)
	s.AuthMode = cfg.AuthMode
	s.APIKey = cfg.APIKey

	log.Printf("JEV API %s:%s (auth_mode=%s)", host, addr, s.AuthMode)
	if err := s.Start(host + ":" + addr); err != nil {
		log.Fatal(err)
	}
}
