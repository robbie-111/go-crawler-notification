package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"go-crawler-notification/handlers"
	"go-crawler-notification/router"
)

// findAvailablePort는 start~end 범위에서 바인딩 가능한 첫 번째 포트를 반환합니다.
func findAvailablePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port in range %d-%d", start, end)
}

func main() {
	// 저장소 초기화
	handlers.InitStores()

	r := router.New()

	port := os.Getenv("PORT")
	if port == "" {
		p, err := findAvailablePort(3001, 3100)
		if err != nil {
			log.Fatalf("Could not find available port: %v", err)
		}
		port = strconv.Itoa(p)
	}

	addr := ":" + port
	log.Printf("Crawler Monitor starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
