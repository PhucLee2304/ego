.PHONY: swag protoc

ifeq ($(OS),Windows_NT)

swag:
	powershell.exe -ExecutionPolicy Bypass -File generate.ps1 swag

protoc:
	powershell.exe -ExecutionPolicy Bypass -File generate.ps1 protoc

else

swag:
	@echo "Generating Swagger docs for all services..."
	@for svc in services/* ; do \
		if [ -d "$$svc" ] && [ -f "$$svc/cmd/main.go" ]; then \
			echo "--> Generating swag for $$svc"; \
			(cd "$$svc" && swag init -g cmd/main.go -o docs --parseDependency --parseInternal) ; \
		fi \
	done

protoc:
	@echo "Generating proto files..."
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

endif
