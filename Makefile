.PHONY: swag protoc build check-ports dev dev-down log test reset-db seed

SERVICE := $(word 2,$(MAKECMDGOALS))

ifeq ($(OS),Windows_NT)

swag:
	powershell.exe -ExecutionPolicy Bypass -File generate.ps1 swag

protoc:
	powershell.exe -ExecutionPolicy Bypass -File generate.ps1 protoc

test:
	powershell.exe -ExecutionPolicy Bypass -File scripts/test.ps1

reset-db:
	$(if $(SERVICE),,$(error Usage: make reset-db SERVICE_NAME))
	powershell.exe -ExecutionPolicy Bypass -File scripts/database.ps1 reset-db $(SERVICE)

seed:
	$(if $(SERVICE),,$(error Usage: make seed SERVICE_NAME))
	powershell.exe -ExecutionPolicy Bypass -File scripts/database.ps1 seed $(SERVICE)

else

swag:
	@echo "Generating Swagger docs for all services..."
	@for svc in services/* ; do \
		if [ -d "$$svc" ] && [ -f "$$svc/cmd/server/main.go" ]; then \
			echo "--> Generating swag for $$svc"; \
			(cd "$$svc" && swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal) ; \
		fi \
	done

protoc:
	@echo "Generating proto files..."
	@mkdir -p api/gen/go
	@for dir in api/proto/* ; do \
		if [ -d "$$dir" ]; then \
			echo "--> Generating proto for $$dir"; \
			protoc -I . -I platform \
				--go_out=./api/gen/go --go_opt=module=ego/api/gen/go \
				--go-grpc_out=./api/gen/go --go-grpc_opt=module=ego/api/gen/go \
				--grpc-gateway_out=./api/gen/go --grpc-gateway_opt=module=ego/api/gen/go \
				$$dir/*.proto ; \
		fi \
	done

test:
	./scripts/test.sh

reset-db:
	$(if $(SERVICE),,$(error Usage: make reset-db SERVICE_NAME))
	SERVICE=$(SERVICE) ACTION=reset-db ./scripts/database.sh

seed:
	$(if $(SERVICE),,$(error Usage: make seed SERVICE_NAME))
	SERVICE=$(SERVICE) ACTION=seed ./scripts/database.sh

endif

check-ports:
	./scripts/check-ports.sh

dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

dev-down:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down

log:
	$(if $(SERVICE),,$(error Usage: make log SERVICE_NAME))
	docker compose logs --follow --tail=200 $(SERVICE)

build:
	docker compose stop $(filter-out $@,$(MAKECMDGOALS))
	./scripts/check-ports.sh
	docker compose up -d --build $(filter-out $@,$(MAKECMDGOALS))
	docker compose restart nginx

%:
	@:
