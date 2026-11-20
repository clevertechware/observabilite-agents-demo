.PHONY: build generate vet test check

build:
	go build -o bin/ctwctl .

generate:
	go generate ./...

vet:
	go vet ./...

test:
	go test ./...

check: generate vet test
	git diff --exit-code -- internal
