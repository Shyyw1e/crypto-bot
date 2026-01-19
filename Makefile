# Все .proto в каталоге proto/
PROTO_FILES := $(wildcard proto/*.proto)

.PHONY: proto
proto:
	protoc -I proto \
		--go_out=. --go-grpc_out=. \
		$(PROTO_FILES)

# Бинарники сервисов
BIN_DIR := bin
SERVICES := analyser rapira-gw tgbot

.PHONY: build
build: $(SERVICES:%=build-%)

build-%:
	go build -o $(BIN_DIR)/$* ./cmd/$*

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	go fmt ./...
