-include Makefile.mk

GOFLAGS=-mod=vendor

PKG := `go list ${GOFLAGS} -f {{.Dir}} ./...`

ifeq ($(RACE),1)
	GOFLAGS+=-race
endif

LINT_VERSION := v2.8.0

MAIN := ${NAME}/cmd/${NAME}

export PGDATABASE
export PGHOST
export PGPORT
export PGUSER
export PGPASSWORD

.PHONY: *

init:
	@cp -n Makefile.mk.dist Makefile.mk
	@cp -n cfg/local.toml.dist cfg/local.toml

show-env:
	@echo "NAME=$(NAME)"
	@echo "TEST_PGDATABASE=$(TEST_PGDATABASE)"
	@echo "PGDATABASE=$(PGDATABASE)"
	@echo "PGHOST=$(PGHOST)"
	@echo "PGPORT=$(PGPORT)"
	@echo "PGUSER=$(PGUSER)"
	@echo "PGPASSWORD=$(PGPASSWORD)"
	@echo "GOFLAGS=$(GOFLAGS)"

tools:
	@go install github.com/vmkteam/mfd-generator@latest
	@go install github.com/vmkteam/pgmigrator@latest
	@go install github.com/vmkteam/colgen/cmd/colgen@latest
	@curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${LINT_VERSION}

fmt:
	@golangci-lint fmt

lint:
	@golangci-lint version
	@golangci-lint config verify
	@golangci-lint run

frontend-install:
	@cd frontend && npm install

frontend-dev:
	@cd frontend && npm run dev

frontend-build:
	@cd frontend && npm run build:all

build:
	@CGO_ENABLED=0 go build $(GOFLAGS) -o ${NAME} $(MAIN)

run:
	@echo "Compiling"
	@go run $(GOFLAGS) $(MAIN) -config=cfg/local.toml -dev

generate:
	@go generate ./pkg/rpc
	@go generate ./pkg/vt

test:
	@echo "Running tests"
	@PGDATABASE=$(TEST_PGDATABASE) go test -count=1 $(GOFLAGS) -coverprofile=coverage.txt -covermode count $(PKG)

test-short:
	@go test $(GOFLAGS) -v -test.short -test.run="Test[^D][^B]" -coverprofile=coverage.txt -covermode count $(PKG)

mod:
	@go mod tidy
	@go mod vendor
	@git add vendor

db:
	@dropdb --if-exists -f $(PGDATABASE)
	@createdb $(PGDATABASE)
	@psql -f docs/$(NAME).sql $(PGDATABASE)
	@psql -f docs/init.sql $(PGDATABASE)

db-test:
	@$(MAKE) --no-print-directory db PGDATABASE=${TEST_PGDATABASE}

NS := "NONE"

mfd-xml:
	@mfd-generator xml -c "postgres://$(PGUSER):$(PGPASSWORD)@$(PGHOST):$(PGPORT)/$(PGDATABASE)?sslmode=disable" -m ./docs/model/$(NAME).mfd
mfd-model:
	@mfd-generator model -m ./docs/model/$(NAME).mfd -p db -o ./pkg/db
mfd-repo: --check-ns
	@mfd-generator repo -m ./docs/model/$(NAME).mfd -p db -o ./pkg/db -n $(NS)
mfd-db-test:
	@mfd-generator dbtest -m docs/model/$(NAME).mfd -o ./pkg/db/test -x $(NAME)/pkg/db
mfd-vt-xml:
	@mfd-generator xml-vt -m ./docs/model/$(NAME).mfd
mfd-vt-rpc: --check-ns
	@mfd-generator vt -m docs/model/$(NAME).mfd -o pkg/vt -p vt -x $(NAME)/pkg/db -n $(NS)
mfd-xml-lang:
	#TODO: add namespaces support for xml-lang command
	@mfd-generator xml-lang  -m ./docs/model/$(NAME).mfd
mfd-vt-template: --check-ns type-script-client
	@mfd-generator template -m docs/model/$(NAME).mfd  -o ../gold-vt/ -n $(NS)

type-script-client: generate
	@go run $(GOFLAGS) $(MAIN) -config=cfg/local.toml -ts_client > ../gold-vt/src/services/api/factory.ts


--check-ns:
ifeq ($(NS),"NONE")
	$(error "You need to set NS variable before run this command. For example: NS=common make $(MAKECMDGOALS) or: make $(MAKECMDGOALS) NS=common")
endif
