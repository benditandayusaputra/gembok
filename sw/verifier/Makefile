DASHBOARD := ../dashboard
WEB := internal/httpapi/web
BIN := bin

.PHONY: all build board test vet check dashboard fixture run clean

all: vet test build

build:
	go build -trimpath -o $(BIN)/gembok-verifier ./cmd/gembok-verifier

board:
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN)/gembok-verifier-arm ./cmd/gembok-verifier

test:
	go test ./...

vet:
	go vet ./...

check: vet test
	GOOS=linux GOARCH=arm GOARM=7 go build ./...

dashboard:
	cd $(DASHBOARD) && npm ci && npm run check && npm run build
	rm -rf $(WEB)
	mkdir -p $(WEB)
	cp -R $(DASHBOARD)/build/. $(WEB)/

fixture:
	cd ../.. && python3 tools/gen_fixture_pemeriksa.py

run:
	go run ./cmd/gembok-verifier --backend sim

clean:
	rm -rf $(BIN)
