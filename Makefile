.PHONY: generate build run dev clean tidy

# templ 코드 생성 (*.templ -> *_templ.go)
generate:
	templ generate

# 빌드
build: generate
	go build -o crawler-monitor .

# 서버 실행 (기본 포트 3001)
run: build
	./crawler-monitor

# 개발 모드: templ watch + go run
dev:
	templ generate --watch &
	PORT=3001 go run .

# 정리
clean:
	rm -f crawler-monitor
	find . -name '*_templ.go' -delete

# Go 모듈 업데이트
tidy:
	go mod tidy
