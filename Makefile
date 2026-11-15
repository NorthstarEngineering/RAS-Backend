.PHONY: fmt check vet test test-coverage integration build build-auth build-branch build-attendance compose-up compose-down logs db-status db-up db-seed db-reset-local terraform-fmt terraform-validate terraform-plan aws-up aws-down aws-shell-down image-auth image-branch image-attendance

fmt:
	cd services/auth-api && go fmt ./...
	cd services/branch-api && go fmt ./...
	cd services/attendance-api && go fmt ./...

check:
	powershell -NoProfile -Command "$$files = Get-ChildItem services -Recurse -Filter *.go; $$bad = foreach ($$f in $$files) { if ((gofmt -l $$f.FullName)) { $$f.FullName } }; if ($$bad) { $$bad; exit 1 }"

vet:
	cd services/auth-api && go vet ./...
	cd services/branch-api && go vet ./...
	cd services/attendance-api && go vet ./...

test:
	cd services/auth-api && go test ./...
	cd services/branch-api && go test ./...
	cd services/attendance-api && go test ./...

test-coverage:
	@powershell -NoProfile -Command "$$ErrorActionPreference = 'Stop'; $$modules = Get-ChildItem services -Recurse -Filter go.mod | Where-Object { $$_.FullName -notmatch '\\\\.tools\\\\' }; New-Item -ItemType Directory -Force test-results | Out-Null; foreach ($$module in $$modules) { $$dir = $$module.DirectoryName; $$name = $$dir.Name; Push-Location $$dir; go test ./... -covermode=atomic -coverprofile=(Join-Path (Join-Path (Get-Location).Path '..\\..\\test-results') ($$name + '.out')); if ($$LASTEXITCODE -ne 0) { Pop-Location; exit $$LASTEXITCODE }; Pop-Location }"

integration:
	docker compose -f deploy/docker-compose.yml exec -T postgres sh -ec 'PGPASSWORD="$$POSTGRES_PASSWORD" psql -h 127.0.0.1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /tests/schema.sql'

build: build-auth build-branch build-attendance
build-auth:
	cd services/auth-api && go build ./cmd/api
build-branch:
	cd services/branch-api && go build ./cmd/api
build-attendance:
	cd services/attendance-api && go build ./cmd/api

compose-up:
	docker compose -f deploy/docker-compose.yml up --build -d
compose-down:
	docker compose -f deploy/docker-compose.yml down
logs:
	docker compose -f deploy/docker-compose.yml logs -f --tail=100

db-status:
	docker compose -f deploy/docker-compose.yml run --rm migrate status
db-up:
	docker compose -f deploy/docker-compose.yml run --rm migrate up
db-seed:
	@test "$${APP_ENV}" = development || (echo "Refusing seed: APP_ENV must equal development" && exit 1)
	docker compose -f deploy/docker-compose.yml exec -T postgres sh -ec 'PGPASSWORD="$$POSTGRES_PASSWORD" psql -h 127.0.0.1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /database/seeds/dev.sql'

db-reset-local:
	@test "$${APP_ENV}" = development || (echo "Refusing reset: APP_ENV must equal development" && exit 1)
	@echo "WARNING: This destroys every row in the disposable local ras_backend_postgres_data volume. Never use it for shared development."
	docker compose -f deploy/docker-compose.yml down -v
	docker compose -f deploy/docker-compose.yml up --build -d

image-auth:
	docker build -t ras-auth-api:local -f services/auth-api/Dockerfile services/auth-api
image-branch:
	docker build -t ras-branch-api:local -f services/branch-api/Dockerfile services/branch-api
image-attendance:
	docker build -t ras-attendance-api:local -f services/attendance-api/Dockerfile services/attendance-api

terraform-fmt:
	terraform -chdir=infra/terraform fmt -check -recursive
terraform-validate:
	terraform -chdir=infra/terraform/dev init -backend=false
	terraform -chdir=infra/terraform/dev validate
terraform-plan:
	terraform -chdir=infra/terraform/dev plan -var-file=terraform.tfvars

aws-up:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\aws-dev.ps1 up

aws-down:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\aws-dev.ps1 down

aws-shell-down:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\aws-dev.ps1 shell-down
