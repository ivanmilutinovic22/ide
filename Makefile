.PHONY: build run fmt vet test clean

build:
	go build -o ./build/ide ./cmd/ide

run:
	go run ./cmd/ide

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -f ide
