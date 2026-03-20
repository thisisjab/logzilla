run flags='':
    - go run ./cmd/engine/main.go {{flags}}

test:
    - go test ./...

build-ui:
    - (cd ./ui && npm run build)
