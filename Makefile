.PHONY: run build test test-unit test-e2e vet lint db-up db-down db-reset migrate migrate-status admin secrets

run:            ## start the API (reads .env)
	go run ./src

build:          ## build the API and admin CLI into bin/
	CGO_ENABLED=0 go build -o bin/pms-api ./src
	CGO_ENABLED=0 go build -o bin/pms-admin ./src/cmd/admin

test:           ## unit + end-to-end tests (e2e needs Docker)
	go test ./...

test-unit:      ## unit tests only
	go test -short ./...

test-e2e:       ## end-to-end tests against a throwaway Postgres 11
	go test -run TestEndToEnd -v ./src/delivery/rest/

vet:
	go vet ./...

lint:           ## needs golangci-lint installed
	golangci-lint run ./...

db-up:          ## start local Postgres 11 with the DBA's pms objects
	docker compose up -d --wait postgres

db-down:
	docker compose down

db-reset:       ## wipe and recreate the local database
	docker compose down -v
	docker compose up -d --wait postgres

migrate:        ## create/upgrade pms.sys_* platform tables
	go run ./src/cmd/admin migrate

migrate-status:
	go run ./src/cmd/admin migrate-status

admin:          ## make admin EMAIL=you@example.com NAME="Your Name"
	go run ./src/cmd/admin create-platform-admin -email "$(EMAIL)" -name "$(NAME)"

secrets:        ## print two fresh JWT secrets for .env
	@echo "JWT_ACCESS_SECRET=$$(openssl rand -base64 48)"
	@echo "JWT_REFRESH_SECRET=$$(openssl rand -base64 48)"
