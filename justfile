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
    go build -o bin/engine ./cmd/engine

build: build-ui build-engine
    @echo "Build completed successfully."
