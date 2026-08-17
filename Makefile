# -- protobuf/grpc parameters ---
PROTO_DIR := api/proto
GRPC_OUT_DIR := pkg/protos/gen

BIN_DIR := bin
TARGET := ${BIN_DIR}/yact

# --- mTLS certs parameters ---
CA_TLS_DIR       := config/certs/ca
CLIENT_TLS_DIR  := config/certs/client
SERVER_TLS_DIR   := config/certs/server
CERTS_CONFIG_DIR := config/certs/config
TLS_CERT_ORG     := WorkerService

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
	go run ./cmd/server \
		-ca ${CA_TLS_DIR}/ca.crt \
		-key ${SERVER_TLS_DIR}/server.key \
		-cert ${SERVER_TLS_DIR}/server.crt

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
build: ## Build yact
	go build -o ${TARGET} ./cmd/

##@ Test
.PHONY: test
test: ## Run tests
	go test -race -v ./pkg/... ./internal/... ./cmd/...

test-integration: create-certs build ## Run integration tests
	bash ./tests/integration.sh

##@ mTLS

.PHONY: make-cert-dir
make-cert-dir:
	mkdir -p \
		${CA_TLS_DIR} \
		${CLIENT1_TLS_DIR} \
		${CLIENT2_TLS_DIR} \
		${CLIENT3_TLS_DIR} \
		${SERVER_TLS_DIR}

.PHONY: create-tls-ca
create-tls-ca: make-cert-dir ## Create mTLS CA
	openssl genpkey -algorithm Ed25519 -out ${CA_TLS_DIR}/ca.key
	openssl req -new -x509 -key ${CA_TLS_DIR}/ca.key -out ${CA_TLS_DIR}/ca.crt \
		-subj "/O=WorkerService/CN=WorkerServiceRootCA" -days 3650

.PHONY: create-tls-client-cert
create-tls-client-cert: ## Create mTLS client certs
	# client 1 - johndoe (admin)
	openssl genpkey -algorithm Ed25519 -out ${CLIENT_TLS_DIR}/client.key
	openssl req -new -key ${CLIENT_TLS_DIR}/client.key -out ${CLIENT_TLS_DIR}/client.csr \
		-subj "/O=WorkerServer/CN=cli-user"
	openssl x509 -req -in ${CLIENT_TLS_DIR}/client.csr -CA ${CA_TLS_DIR}/ca.crt \
		-CAkey ${CA_TLS_DIR}/ca.key -CAcreateserial -out ${CLIENT_TLS_DIR}/client.crt -days 365 \
		-extensions v3_req -extfile ${CERTS_CONFIG_DIR}/client.conf

.PHONY: create-tls-server-cert
create-tls-server-cert: ## Create mTLS server certs
	# server
	openssl genpkey -algorithm Ed25519 -out ${SERVER_TLS_DIR}/server.key
	openssl req -new -key ${SERVER_TLS_DIR}/server.key -out ${SERVER_TLS_DIR}/server.csr \
		-subj "/O=${TLS_CERT_ORG}/CN=localhost"
	openssl x509 -req -in ${SERVER_TLS_DIR}/server.csr -CA ${CA_TLS_DIR}/ca.crt \
		-CAkey ${CA_TLS_DIR}/ca.key -CAcreateserial -out ${SERVER_TLS_DIR}/server.crt -days 365 \
		-extensions v3_req -extfile ${CERTS_CONFIG_DIR}/server.conf

.PHONY: create-certs
create-certs: create-tls-ca create-tls-client-cert create-tls-server-cert ## Create all mTLS certs

clean-certs: ## Clean certs
	-rm -rf \
		./${CA_TLS_DIR} \
		./${CLIENT1_TLS_DIR} \
		./${CLIENT2_TLS_DIR} \
		./${CLIENT3_TLS_DIR} \
		./${SERVER_TLS_DIR}
	-rm -rf ./tests/output
