# grpc-training

Go projects for testing gRPC integrations.

## LifeSure — a life insurance platform (main example)

A fictional life insurance platform split into four independent gRPC
services, chained together by a demo client. It exists to exercise the
things a single client/server pair can't show: multiple services, all four
RPC shapes, shared interceptors, and calls that cross service boundaries.

| Service                | RPC shape              | Method(s) |
|-------------------------|-------------------------|-----------|
| `QuoteService`          | unary                   | `GetQuote` — price a policy from age/smoker/coverage/term |
| `UnderwritingService`   | client-streaming        | `SubmitMedicalHistory` — stream medical records, get one final decision |
| `PolicyService`         | unary + server-streaming| `CreatePolicy`, `GetPolicy`, `ListPolicies` (streamed) |
| `ClaimsService`         | bidirectional streaming | `ProcessClaim` — stream claim events, get status updates back live |

The demo client (`cmd/demo`) walks one applicant through the whole
lifecycle: get a quote → submit medical history for underwriting → issue the
policy if approved → list the holder's policies → file and track a claim.

Cross-cutting pieces shared by all four services:

- `proto/common/common.proto` — shared `Money` and `RiskTier` types
- `internal/authkey` — a server interceptor (unary + streaming) that rejects
  calls missing an `x-api-key` metadata value, plus client-side
  `credentials.PerRPCCredentials` that attaches it
- `internal/interceptor` — a logging interceptor (unary + streaming) that
  logs method, duration, and gRPC status code
- Every server registers gRPC reflection, so you can explore them with
  `grpcurl` without writing a client

### Running it

Each service is its own binary. Start all four (in separate terminals, or
backgrounded), then run the demo client:

```sh
make quoteserver         # :50051
make underwritingserver  # :50052
make policyserver        # :50053
make claimsserver        # :50054

make demo                # runs the full flow against all four
```

Regenerate protobuf code after editing anything under `proto/`:

```sh
make proto-lifesure
```

### Layout

```
proto/                  service + shared .proto definitions
gen/pb/                 generated protobuf/gRPC Go code
internal/authkey/       API-key auth (server interceptors + client credentials)
internal/interceptor/   logging interceptors
internal/idgen/         short random ID generator for demo entities
cmd/quoteserver/        QuoteService binary
cmd/underwritingserver/ UnderwritingService binary
cmd/policyserver/       PolicyService binary
cmd/claimsserver/       ClaimsService binary
cmd/demo/               orchestrator client exercising the full flow
```

## examples/basic-greeter — minimal starting point

A single `Greeter` service (unary `SayHello` + server-streaming
`SayHelloStream`) with one server and one client. Useful as a smaller
reference before diving into LifeSure.

```sh
make greeter-server
make greeter-client
```

Regenerate its protobuf code with `make proto-greeter`.
