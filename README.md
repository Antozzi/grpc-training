# grpc-training

A minimal Go gRPC project for testing gRPC integrations: a `Greeter` service
with both a unary RPC (`SayHello`) and a server-streaming RPC (`SayHelloStream`).

## Layout

- `proto/greeter.proto` — service definition
- `gen/pb/` — generated protobuf/gRPC Go code
- `server/` — gRPC server implementation
- `client/` — gRPC client implementation

## Usage

Regenerate protobuf code after editing the `.proto` file:

```sh
make proto
```

Run the server:

```sh
make server
```

In another terminal, run the client:

```sh
make client
```

Or with a custom name/address:

```sh
go run ./client -name Anton -addr localhost:50051
```
