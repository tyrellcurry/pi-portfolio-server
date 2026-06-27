.PHONY: run webserver metrics build clean

run:
	@trap 'kill 0' INT TERM EXIT; \
	go run ./cmd/webserver & \
	go run ./cmd/metrics & \
	wait

webserver:
	go run ./cmd/webserver

metrics:
	go run ./cmd/metrics

build:
	go build -o bin/webserver ./cmd/webserver
	go build -o bin/metrics   ./cmd/metrics

clean:
	rm -rf bin
