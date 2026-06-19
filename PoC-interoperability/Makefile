APP=firewall-backend

.PHONY: deps generate build run clean

deps:
	go mod tidy

generate:
	go generate ./...

build:
	mkdir -p bin
	go build -o bin/$(APP) ./cmd/server

run: build
	./bin/$(APP)

clean:
	rm -rf bin
