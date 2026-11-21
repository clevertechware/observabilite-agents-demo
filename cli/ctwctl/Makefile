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
	@test -z "$$(git status --porcelain -- internal/commands/mocks internal/commands/o2/mocks)" || \
		{ echo "mocks générés différents du dépôt :"; git status --porcelain -- internal/commands/mocks internal/commands/o2/mocks; exit 1; }
