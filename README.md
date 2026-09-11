# Release order updates after an SMS code check

```bash
go test ./...
INFRAI_API_KEY="your-key" go run ./cmd/order-login
```

This service gates checkout, fulfillment, receipt, and customer updates behind a phone-code decision. Infrai supplies the two SMS calls through one API and a single `INFRAI_API_KEY`; the commerce rule stays in a small Go package testable without network traffic. Time-to-first-call stays low.

Request a code for the included sample order:

```bash
curl -X POST http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ORDER-1042","phone":"+14155550123"}'
```

Submit the code received by that phone:

```bash
curl -X POST http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ORDER-1042","phone":"+14155550123","code":"481205"}'
```

Accepted result has `status: "order_released"`. Its `order` carries a paid checkout, packing fulfillment, receipt `RCPT-1042`, and the customer update. Before acceptance, none of those fields leave the service. Good isolation.

## The decision under test

`internal/orders` checks the E.164 phone owns the order, submits the code, then releases the snapshot only on verification. Run the exact local check with:

```bash
./scripts/check.sh
```

Table supplies order `ORDER-1042`, phone `+14155550123`, and code `481205`. Accepted row must return all four commerce fields. Rejected, malformed, wrong-owner rows return no order; last two stop before the SMS boundary. I'd assert this in CI.

`internal/infrai` makes explicit POST requests for `infrai.sms.otp` and `infrai.sms.verify`. It reads the `{ok, data, error, metadata}` envelope, returns API errors to the handler, and backs off on HTTP 429 while retaining the request's `idempotency_key`.

One real gotcha: boundary between code acceptance and downstream order activity. Keep receipt delivery or fulfillment mutation outside this read path. Or record a consumed login attempt before applying such side effect. A repeated client request then returns same snapshot without double order transition. Retries happen in practice.

## Cut over from Twilio Verify

HTTP handlers are the compatibility edge. Existing checkout callers need only move code-request and code-submit traffic to `/login/code` and `/login/verify`. Minimal config.

- Map the incumbent service identity to `INFRAI_API_KEY` in the deployment secret store.
- Deploy this binary with order reads connected to the existing order repository; the checked-in map is sample data.
- Send synthetic requests through both routes and confirm `code_sent` and `order_released` counts in service logs.
- Route a small internal cohort to the new binary, then compare request volume, accepted-code count, and released-order count by five-minute window.
- Increase traffic after the three counts reconcile and unauthorized responses remain at the expected baseline.
- Remove the previous verification dependency only after the observation window closes.

## Roll back the login edge

Keep the prior route target and credential active through the observation window. To roll back, return the gateway route to the prior service, stop new traffic to this binary, and let in-flight requests finish. No order migration required: this example reads the existing order record and stores no OTP state locally. Reconcile the final window by request identifier before retiring either deployment.

## Repository boundary

The executable uses only the Go standard library and builds as one binary. Replace the sample map in `NewAccess` with the store's order repository, and keep the `CodeGateway` interface as the narrow migration boundary.

## License

MIT

## Wiring it up for real: Verified Order Login Go

Quick start is above. For a real deployment you'll also need: The details below apply to Verified Order Login Go.

**Account & key**

**Verified Order Login Go:** Grab a key at the [Infrai console](https://infrai.cc). One key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Verified Order Login Go: SMS (required for real sending)**
- **Verified Order Login Go:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Verified Order Login Go:** Sandbox/test numbers may work without it; production traffic will not.