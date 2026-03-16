run flags='':
    - go run ./cmd/engine/main.go {{flags}}

test:
    - go test ./...
