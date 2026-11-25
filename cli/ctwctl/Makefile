.PHONY: build generate vet lint test check

build:
	go build -o bin/ctwctl .

generate:
	go generate ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

test:
	go test ./...

check: generate vet lint test
	@test -z "$$(git status --porcelain -- internal/app/mocks internal/order/mocks internal/commands/o2/mocks)" || \
		{ echo "mocks générés différents du dépôt :"; git status --porcelain -- internal/app/mocks internal/order/mocks internal/commands/o2/mocks; exit 1; }
