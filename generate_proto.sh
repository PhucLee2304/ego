# bash generate_proto.sh
protoc.exe -I . -I platform --go_out=./api/gen/go --go_opt=module=ego/api/gen/go --go-grpc_out=./api/gen/go --go-grpc_opt=module=ego/api/gen/go --grpc-gateway_out=./api/gen/go --grpc-gateway_opt=module=ego/api/gen/go api/proto/identity/identity.proto
protoc.exe -I . -I platform --go_out=./api/gen/go --go_opt=module=ego/api/gen/go --go-grpc_out=./api/gen/go --go-grpc_opt=module=ego/api/gen/go --grpc-gateway_out=./api/gen/go --grpc-gateway_opt=module=ego/api/gen/go api/proto/storage/storage.proto
