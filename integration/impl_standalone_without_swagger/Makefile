FIRST_GOPATH:=$(firstword $(subst :, ,$(GOPATH)))
GRPC_GATEWAY_PATH?=${FIRST_GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway

clean:
	find . -regex "\.\/.*\/.*\.go" -exec rm {} +
	rm -f main

protoc:
	protoc -I/usr/local/include:${GRPC_GATEWAY_PATH}/third_party/googleapis:. --go_out=plugins=grpc:. --goclay_out=swagger=false,impl=true,impl_path=../strings:. pb/strings.proto

build:
	go build -o main main.go && echo

test: clean protoc build