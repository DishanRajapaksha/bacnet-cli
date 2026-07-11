.PHONY: build test vet fmt check

build:
	go build -o bacnet-cli .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

check: fmt vet test build
