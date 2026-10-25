# -- protobuf/grpc parameters ---
PROTO_DIR := api/proto
GRPC_OUT_DIR := pkg/protos/gen

BIN_DIR := bin
TARGET  := ${BIN_DIR}/yact
ARCH    ?= ${shell uname -m | sed 's/x86_64/amd64/'}

BRANCH   := ${shell git rev-parse --abbrev-ref HEAD}
REVCNT   := ${shell git rev-list --count $(BRANCH)}
REVHASH  := ${shell git log -1 --format="%h"}
LDFLAGS  := -X main.version=${BRANCH}.${REVCNT}.${REVHASH} -s
CFLAGS   := --ldflags '${LDFLAGS}'

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ \
		{ printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ \
		{ printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


.PHONY: init-hooks
init-hooks: ## init git hooks
	cp -f gitHooks/* .git/hooks/
	chmod +x .git/hooks/*


clean: clean-certs ## Clean everything
	-rm -rf ./${BIN_DIR}

##@ Run

.PHONY: run-server
run-server: ## Run the server
	go run ./cmd proxy \
		--cert-dir ./config/certs/server

##@ Development

.PHONY: generate
generate: generate-protos ## Generate all

.PHONY: make-proto-dir
make-proto-dir:
	mkdir -p ${GRPC_OUT_DIR}

.PHONY: generate-protos
generate-protos: make-proto-dir  ## Generate protos
	protoc \
		-I ${PROTO_DIR} \
		--go_out=${GRPC_OUT_DIR} \
		--go_opt=paths=source_relative \
		--go-grpc_out=${GRPC_OUT_DIR} \
		--go-grpc_opt=paths=source_relative \
		$(shell find ${PROTO_DIR} -name '*.proto')

.PHONY: lint
lint: lint-proto lint-go ## Lint

.PHONY: lint-proto
lint-proto: ## Lint protos
	buf lint

.PHONY: lint-go
lint-go: ## Lint go
	golangci-lint run

##@ Build
.PHONY: build
build: ## Build yact for current os/arch
	go build ${CFLAGS} -o ${TARGET} ./cmd/

.PHONY: build/linux
build/linux: clean ## Build linux binary of yact
	CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build ${CFLAGS} -o ${TARGET}_linux_${ARCH} ./cmd

##@ Test
.PHONY: test
test: ## Run tests
	go test -race -v ./pkg/... ./internal/... ./cmd/...

test-integration: create-certs build ## Run integration tests
	bash ./tests/integration.sh

##@ mTLS

.PHONY: create-certs
create-certs: ## Create mTLS certs
	go run ./cmd/ make-certs --cert-dir ./config/certs

clean-certs: ## Clean certs
	-rm -rf ./config/certs
	-rm -rf ./tests/output
