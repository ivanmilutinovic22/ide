.PHONY: build run fmt vet test clean

build:
	go build -o ./build/ide .

run:
	go run .

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

clean:
	rm -f ide
