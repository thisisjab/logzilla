run flags='':
    - go run ./cmd/engine/main.go {{flags}}

test:
    - go test ./...

build-ui:
    - (cd ./ui && npm run build)

build-engine:
    go build -o bin/engine ./cmd/engine

build: build-ui build-engine
    @echo "Build completed successfully."
