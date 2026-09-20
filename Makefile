.PHONY: proto server client

proto:
	protoc --go_out=. --go_opt=module=github.com/Antozzi/grpc-training \
		--go-grpc_out=. --go-grpc_opt=module=github.com/Antozzi/grpc-training \
		proto/greeter.proto

server:
	go run ./server

client:
	go run ./client
