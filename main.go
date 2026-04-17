package main

import (
	"log"
	"net/http"
	"os"

	"go-crawler-notification/handlers"
	"go-crawler-notification/router"
)

func main() {
	// 저장소 초기화
	handlers.InitStores()

	r := router.New()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	addr := ":" + port
	log.Printf("Crawler Monitor starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
