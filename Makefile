.PHONY: all build test clean run web-install web-build lint

BINARY_NAME=multihop

all: build

web-install:
	cd web && npm install

web-build:
	cd web && npm run build

build:
	go build -o $(BINARY_NAME) ./cmd/multihop

build-prod: web-build
	go build -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/multihop

test:
	go test -v -race ./...

bench:
	go test -bench=. -benchmem ./internal/...

clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME).exe
	rm -f *.db *.db-wal *.db-shm
	rm -rf web/dist

run: build
	./$(BINARY_NAME) serve --port 8080 --db knowledge.db
