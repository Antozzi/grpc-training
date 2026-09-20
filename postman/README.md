# Testing LifeSure in Postman

## A note on what's in this folder

Postman auto-created `.postman/` and `postman/postman/` here as part of its
"Local Workspace" git-sync feature (linked to a real cloud workspace ID).
That sync format is new and its schema for gRPC requests isn't publicly
documented, so rather than risk writing files there that fail to load — or
that corrupt the sync back to your actual Postman workspace — this guide
uses the plain, well-documented **Postman Environment** import instead, plus
step-by-step instructions for building the gRPC requests directly in
Postman's UI. It's a few minutes of clicking, but every step below is
something Postman's own docs confirm works.

## 1. Start the four services

```sh
make quoteserver         # :50051
make underwritingserver  # :50052
make policyserver        # :50053
make claimsserver        # :50054
```

## 2. Import the environment

In Postman: **File → Import** → select `postman/LifeSure.postman_environment.json`
→ then pick **LifeSure Local** from the environment dropdown (top right).

It defines `{{quote_addr}}`, `{{underwriting_addr}}`, `{{policy_addr}}`,
`{{claims_addr}}`, `{{api_key}}`, `{{holder_id}}`, `{{holder_name}}`, and
empty `{{quote_id}}` / `{{policy_id}}` / `{{claim_id}}` slots you'll fill in
by hand as you go (click the eye icon next to the environment dropdown →
edit → paste the ID from a response, since there's no collection scripting
here to do it for you).

## 3. Create each gRPC request

For every request: **New → gRPC**. Because all four servers register gRPC
reflection, Postman auto-discovers the service and its methods as soon as
you enter the address and click **Select a method** — no `.proto` import
needed. Add `x-api-key` / `{{api_key}}` on the **Metadata** tab of every
request (all four services reject calls without it). Put the message JSON
on the **Message** tab.

Note: our `.proto` fields are `snake_case`, but protobuf's JSON mapping
(which Postman follows) renders them as `lowerCamelCase` — that's why the
JSON below uses `coverageAmount`, not `coverage_amount`.

### QuoteService.GetQuote — unary

- Address: `{{quote_addr}}`, service `lifesure.quote.QuoteService`, method `GetQuote`
- Message tab, then **Invoke**:

```json
{
  "age": 45,
  "smoker": false,
  "coverageAmount": { "currencyCode": "USD", "minorUnits": 25000000 },
  "termYears": 20
}
```

Copy the returned `quoteId` and `monthlyPremium.minorUnits` into the
`quote_id` environment variable (and note the premium — you'll need it for
the next step).

### UnderwritingService.SubmitMedicalHistory — client-streaming

- Address: `{{underwriting_addr}}`, method `SubmitMedicalHistory`
- Click **Invoke** first, then use **Send** to stream each message below,
  and **End Streaming** after the last one to get the single decision back.

First message (proto3 JSON represents a `oneof` by including whichever
branch is set, by its own field name — not wrapped in `payload`):

```json
{
  "context": {
    "quoteId": "<paste quote_id>",
    "age": 45,
    "smoker": false,
    "baseMonthlyPremium": { "currencyCode": "USD", "minorUnits": <paste from GetQuote> },
    "baseRiskTier": "RISK_TIER_STANDARD"
  }
}
```

Then send these three, one **Send** click each:

```json
{ "record": { "type": "RECORD_TYPE_CONDITION", "description": "controlled hypertension", "severity": 3 } }
```
```json
{ "record": { "type": "RECORD_TYPE_MEDICATION", "description": "lisinopril, daily", "severity": 2 } }
```
```json
{ "record": { "type": "RECORD_TYPE_LIFESTYLE", "description": "non-smoker, occasional alcohol", "severity": 1 } }
```

Click **End Streaming** — the response pane shows the `UnderwritingDecision`
with `approved`, `finalRiskTier`, and `adjustedMonthlyPremium`.

### PolicyService.CreatePolicy — unary

- Address: `{{policy_addr}}`, method `CreatePolicy`

```json
{
  "quoteId": "<quote_id>",
  "holderId": "{{holder_id}}",
  "holderName": "{{holder_name}}",
  "monthlyPremium": { "currencyCode": "USD", "minorUnits": <adjustedMonthlyPremium.minorUnits from the decision> },
  "coverageAmount": { "currencyCode": "USD", "minorUnits": 25000000 },
  "termYears": 20
}
```

Copy the returned `policyId` into the `policy_id` environment variable.

### PolicyService.GetPolicy — unary

```json
{ "policyId": "{{policy_id}}" }
```

### PolicyService.ListPolicies — server-streaming

- Method `ListPolicies`, click **Invoke** once:

```json
{ "holderId": "{{holder_id}}" }
```

Responses stream into the pane as each matching policy is sent (there's a
small artificial delay per policy, so you can watch them arrive).

### ClaimsService.ProcessClaim — bidirectional streaming

- Address: `{{claims_addr}}`, method `ProcessClaim`
- Click **Invoke**, then **Send** each event below in order — a matching
  `ClaimStatusUpdate` arrives after each one:

```json
{ "policyId": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_OPENED", "detail": "hospitalization claim" }
```
```json
{ "policyId": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_DOCUMENT_SUBMITTED", "detail": "discharge summary" }
```
```json
{ "policyId": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_DOCUMENT_SUBMITTED", "detail": "itemized invoice" }
```
```json
{ "policyId": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_CLOSED", "detail": "all documents submitted" }
```

The first response carries the server-assigned `claimId` — copy it into the
`claim_id` variable if you want to reference it elsewhere. Click **End
Streaming** when done.

## Trying failure cases

- Remove the `x-api-key` metadata entry on any request → expect
  `UNAUTHENTICATED`.
- `GetQuote` with `age: 15` → expect `INVALID_ARGUMENT`.
- `GetPolicy` with a made-up `policyId` → expect `NOT_FOUND`.
- Send only one document before `CLAIM_EVENT_TYPE_CLOSED` → the claim comes
  back `CLAIM_STAGE_DENIED` instead of `CLAIM_STAGE_APPROVED`.
