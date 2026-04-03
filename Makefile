PORT ?= 8080

run:
	PORT=$(PORT) go run .

test:
	go test ./...
