.PHONY: build run clean test

build:
	go build -o bpos_cr_monitor

run:
	go run . -config config.yaml

clean:
	rm -f bpos_cr_monitor
	rm -rf web/

test:
	go test ./...

deps:
	go mod download
	go mod tidy

