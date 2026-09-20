.PHONY: proto proto-lifesure proto-greeter quoteserver underwritingserver policyserver claimsserver demo greeter-server greeter-client

# Regenerate everything.
proto: proto-lifesure proto-greeter

proto-lifesure:
	protoc -I proto \
		--go_out=gen/pb --go_opt=module=github.com/Antozzi/grpc-training/gen/pb \
		--go-grpc_out=gen/pb --go-grpc_opt=module=github.com/Antozzi/grpc-training/gen/pb \
		proto/common/common.proto proto/quote/quote.proto proto/underwriting/underwriting.proto \
		proto/policy/policy.proto proto/claims/claims.proto

proto-greeter:
	protoc -I examples/basic-greeter/proto \
		--go_out=examples/basic-greeter/gen/pb --go_opt=paths=source_relative \
		--go-grpc_out=examples/basic-greeter/gen/pb --go-grpc_opt=paths=source_relative \
		examples/basic-greeter/proto/greeter.proto

# LifeSure services (run each in its own terminal, then `make demo`).
quoteserver:
	go run ./cmd/quoteserver

underwritingserver:
	go run ./cmd/underwritingserver

policyserver:
	go run ./cmd/policyserver

claimsserver:
	go run ./cmd/claimsserver

demo:
	go run ./cmd/demo

# Basic greeter example.
greeter-server:
	go run ./examples/basic-greeter/server

greeter-client:
	go run ./examples/basic-greeter/client
