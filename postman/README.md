# Testing LifeSure in Postman

## The story: what you're actually testing

Meet **Anton**. He's 45, doesn't smoke, and wants a life insurance policy
that pays out $250,000 to his family if something happens to him, for the
next 20 years. Here's what happens behind the scenes, and which gRPC call
does each part — this is the same story `make demo` runs end to end, just
walked through by hand in Postman instead:

1. **"How much would this cost me?"** Anton fills out a quote form: his
   age, whether he smokes, how much coverage he wants, for how long. The
   insurer prices it instantly and gives him a number — **but it's not
   final yet**, because they haven't looked at his health.
   → `QuoteService.GetQuote`

2. **"Now tell us about your health."** Before the insurer commits to that
   price, they need Anton's medical history: any conditions, medications,
   lifestyle factors. He sends these one at a time (like filling out a
   multi-page medical form), and once he's done, the underwriter reviews
   everything at once and comes back with a final decision — approved or
   declined, and the price adjusted up if he's riskier than the initial
   guess assumed.
   → `UnderwritingService.SubmitMedicalHistory`

3. **"You're approved — here's your policy."** With underwriting done, the
   insurer actually issues the policy: a real contract with a policy
   number, at the agreed price.
   → `PolicyService.CreatePolicy`

4. **"Let me check my policy."** Later, Anton (or the insurer's support
   staff) can look up that one policy, or list every policy he holds.
   → `PolicyService.GetPolicy`, `PolicyService.ListPolicies`

5. **"I need to make a claim."** Eight months later, Anton is hospitalized.
   He opens a claim, uploads his discharge summary and an itemized
   invoice, then tells the insurer he's submitted everything. The whole
   time, the insurer's system is pushing him live status updates —
   received, under review, and finally approved for payout (or denied, if
   he hadn't provided enough documentation).
   → `ClaimsService.ProcessClaim`

Each step below is one of these calls, in the same order, with the exact
address and message to send.

## A note on what's in this folder

Postman auto-created `.postman/` and `postman/postman/` here as part of its
"Local Workspace" git-sync feature (linked to a real cloud workspace ID).
That sync format is new and its schema for gRPC requests isn't publicly
documented, so rather than risk writing files there that fail to load — or
that corrupt the sync back to your actual Postman workspace — this guide
uses the plain, well-documented **Postman Environment** import instead, plus
step-by-step instructions for building the gRPC requests directly in
Postman's UI. It's a few minutes of clicking, but every step below has been
run against the live servers and confirmed working.

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

## Whole-flow runbook (condensed)

The full end-to-end sequence, all six requests, one screen — useful as a
checklist, or to hand to an AI agent rebuilding the collection by hand from
the detailed steps further down:

1. `QuoteService.GetQuote` → save `quote_id`, `monthly_premium.minor_units`.
2. `UnderwritingService.SubmitMedicalHistory` → Invoke with `context` (using
   the two values above), `Send` three `record` messages, `End Streaming` →
   save `adjusted_monthly_premium.minor_units`. If `approved: false`, stop.
3. `PolicyService.CreatePolicy` (using `quote_id` + the adjusted premium) →
   save `policy_id`.
4. `PolicyService.GetPolicy` and `PolicyService.ListPolicies` (using
   `policy_id` / `holder_id`) → read-only checks, nothing to carry forward.
5. `ClaimsService.ProcessClaim` → Invoke with `OPENED` → save `claim_id`,
   `Send` two `DOCUMENT_SUBMITTED` events, `Send` `CLOSED` → expect
   `CLAIM_STAGE_APPROVED`, `End Streaming`.

Exact bodies and addresses for each step are in the sections below.

**Fully automated alternative:** `make demo` (`cmd/demo/main.go`) already
runs this entire sequence as real Go code against the same four servers —
confirmed working end-to-end. Reach for that instead of the manual Postman
flow when you just need to exercise the whole thing, not inspect it request
by request.

**Auto-chaining variables inside Postman itself:** gRPC requests do have a
**Scripts** tab with `Before invoke` / `On message` / `After response`
hooks, and `pm.environment.set(...)` works there same as HTTP — see
[Postman's gRPC scripting docs](https://learning.postman.com/docs/sending-requests/grpc/scripting-in-grpc-request)
and [test examples](https://learning.postman.com/docs/sending-requests/grpc/test-examples/).
Responses are read via `pm.response.messages` (e.g.
`pm.response.messages.idx(0).data`), not `pm.response.json()` — that part's
confirmed from the docs, but the exact accessor for a single unary
response's fields wasn't fully documented anywhere I could verify, so treat
it as a starting point to confirm live in the app, not copy-paste-ready.

## Why there's no importable collection file here

A hand-authored collection JSON (guessing at Postman's internal schema for
saved gRPC requests, which isn't publicly documented) was tried and
confirmed broken: it imports without error and *looks* right — shows up as
a "GRPC" request, body pre-filled — but sending it fails, because Postman's
own error messages show the imported item falls back to its plain **HTTP**
request engine underneath ("Postman's HTTP request editor expects a
standard HTTP/HTTPS URL"). It has no real service/method binding or
protobuf framing, so it can't invoke a gRPC call no matter what goes in the
address field. If you have that collection lying around, delete it — it's
not salvageable. The manual steps below are confirmed working against the
live servers instead.

## Build each request by hand

For every request: **New → gRPC**. Because all four servers register gRPC
reflection, Postman auto-discovers the service and its methods as soon as
you enter the address and click **Select a method** — no `.proto` import
needed. Add `x-api-key` / `{{api_key}}` on the **Metadata** tab of every
request (all four services reject calls without it). Put the message JSON
on the **Message** tab.

**Field naming:** unlike `grpcurl`/most protobuf-JSON libraries (which
convert to `lowerCamelCase`), Postman's gRPC client builds its message
editor straight from the reflected proto descriptors and uses the exact
`snake_case` field names as declared in the `.proto` files — confirmed by
clicking **Use Example Message** in the app. Every body below uses
`snake_case` to match.

### QuoteService.GetQuote — unary

- Address: `{{quote_addr}}`, service `lifesure.quote.QuoteService`, method `GetQuote`
- Message tab, then **Invoke**:

```json
{
  "age": 45,
  "smoker": false,
  "coverage_amount": { "currency_code": "USD", "minor_units": "25000000" },
  "term_years": 20
}
```

Response looks like:

```json
{
  "quote_id": "quo-e89248fa",
  "monthly_premium": { "currency_code": "USD", "minor_units": "13956" },
  "risk_tier": "RISK_TIER_STANDARD",
  "expires_at": "2026-10-20T16:33:48Z"
}
```

Copy `quote_id` into the `quote_id` environment variable, and note
`monthly_premium.minor_units` — you'll need it for the next step.

### UnderwritingService.SubmitMedicalHistory — client-streaming

- Address: `{{underwriting_addr}}`, method `SubmitMedicalHistory`
- Click **Invoke** first, then use **Send** to stream each message below,
  and **End Streaming** after the last one to get the single decision back.

First message (proto3 JSON represents a `oneof` by including whichever
branch is set, by its own field name — not wrapped in `payload`):

```json
{
  "context": {
    "quote_id": "<paste quote_id>",
    "age": 45,
    "smoker": false,
    "base_monthly_premium": { "currency_code": "USD", "minor_units": "<paste monthly_premium.minor_units from GetQuote>" },
    "base_risk_tier": "RISK_TIER_STANDARD"
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

Click **End Streaming** — the response pane shows the `UnderwritingDecision`:

```json
{
  "quote_id": "quo-e89248fa",
  "approved": true,
  "final_risk_tier": "RISK_TIER_STANDARD",
  "adjusted_monthly_premium": { "currency_code": "USD", "minor_units": "16188" },
  "notes": "minor risk factors found; premium adjusted"
}
```

### PolicyService.CreatePolicy — unary

- Address: `{{policy_addr}}`, method `CreatePolicy`

```json
{
  "quote_id": "<quote_id>",
  "holder_id": "{{holder_id}}",
  "holder_name": "{{holder_name}}",
  "monthly_premium": { "currency_code": "USD", "minor_units": "<adjusted_monthly_premium.minor_units from the decision>" },
  "coverage_amount": { "currency_code": "USD", "minor_units": "25000000" },
  "term_years": 20
}
```

Copy the returned `policy_id` into the `policy_id` environment variable.

### PolicyService.GetPolicy — unary

```json
{ "policy_id": "{{policy_id}}" }
```

### PolicyService.ListPolicies — server-streaming

- Method `ListPolicies`, click **Invoke** once:

```json
{ "holder_id": "{{holder_id}}" }
```

Responses stream into the pane as each matching policy is sent (there's a
small artificial delay per policy, so you can watch them arrive).

### ClaimsService.ProcessClaim — bidirectional streaming

- Address: `{{claims_addr}}`, method `ProcessClaim`
- Click **Invoke**, then **Send** each event below in order — a matching
  `ClaimStatusUpdate` arrives after each one:

```json
{ "policy_id": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_OPENED", "detail": "hospitalization claim" }
```
```json
{ "policy_id": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_DOCUMENT_SUBMITTED", "detail": "discharge summary" }
```
```json
{ "policy_id": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_DOCUMENT_SUBMITTED", "detail": "itemized invoice" }
```
```json
{ "policy_id": "{{policy_id}}", "type": "CLAIM_EVENT_TYPE_CLOSED", "detail": "all documents submitted" }
```

The first response carries the server-assigned `claim_id` — copy it into
the `claim_id` variable if you want to reference it elsewhere. Click **End
Streaming** when done.

## Trying failure cases

- Remove the `x-api-key` metadata entry on any request → expect
  `UNAUTHENTICATED`.
- `GetQuote` with `age: 15` → expect `INVALID_ARGUMENT`.
- `GetPolicy` with a made-up `policy_id` → expect `NOT_FOUND`.
- Send only one document before `CLAIM_EVENT_TYPE_CLOSED` → the claim comes
  back `CLAIM_STAGE_DENIED` instead of `CLAIM_STAGE_APPROVED`.
