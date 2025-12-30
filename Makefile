.PHONY: build run clean test create-keystore

build:
	go build -o bpos_cr_monitor

run:
	@echo "Usage: make run PASSWORD='your_keystore_password'"
	@if [ -z "$(PASSWORD)" ]; then \
		echo "Error: PASSWORD is required. Example: make run PASSWORD='your_password'"; \
		exit 1; \
	fi
	go run . -config config.json -p "$(PASSWORD)"

create-keystore:
	@echo "Creating keystore..."
	@go run ./cmd/create_keystore

clean:
	rm -f bpos_cr_monitor
	rm -rf web/

test:
	go test ./...

deps:
	go mod download
	go mod tidy

