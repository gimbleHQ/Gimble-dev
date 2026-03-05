APP := gimble
VERSION ?= 0.1.10
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build build-linux build-macos package-deb clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/$(APP)

build-linux:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-linux-amd64 ./cmd/$(APP)
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-linux-arm64 ./cmd/$(APP)

build-macos:
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-darwin-amd64 ./cmd/$(APP)
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-darwin-arm64 ./cmd/$(APP)

package-deb: build-linux
	./scripts/package-deb.sh $(VERSION) amd64
	./scripts/package-deb.sh $(VERSION) arm64

clean:
	rm -rf bin dist .pkg
