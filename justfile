run-engine flags='': build-ui
    - go run ./cmd/engine/main.go {{flags}}

run-ui flags='':
    - (cd ./ui && npm run dev {{flags}})

test-engine:
    - go test ./...

test-ui:
    - (cd ./ui && npm run test)

test: test-engine test-ui

build-ui:
    - (cd ./ui && npm run build)

build-engine:
    @echo "Building for linux..."
    GOOS=linux GOARCH=amd64 go build -o bin/engine-linux-64 ./cmd/engine
    @echo "Building for windows..."
    GOOS=windows GOARCH=amd64 go build -o bin/engine-windows-64.exe ./cmd/engine
    @echo "Building for darwin..."
    GOOS=darwin GOARCH=amd64 go build -o bin/engine-darwin-64 ./cmd/engine
    @echo "Engine is built successfully."

build: build-ui build-engine
    @echo "Build completed successfully."
