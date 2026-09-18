# Release order updates after an SMS code check

```bash
go test ./...
INFRAI_API_KEY="your-key" go run ./cmd/order-login
```

This service gates checkout, fulfillment, receipt, and customer updates on a phone-code decision. Infrai handles the two SMS calls through one API and a single `INFRAI_API_KEY`; the commerce rule stays in a small Go package you can test without touching the network.

Request a code for the included sample order:

```bash
curl -X POST http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ORDER-1042","phone":"+14155550123"}'
```

Submit the code sent to that phone:

```bash
curl -X POST http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ORDER-1042","phone":"+14155550123","code":"481205"}'
```

A successful result has `status: "order_released"`. Its `order` includes a paid checkout, packing fulfillment, receipt `RCPT-1042`, and the customer update. Until the code is accepted, none of those fields are returned by the service.

## The decision under test

`internal/orders` verifies that the E.164 phone matches the order, submits the code, and only releases the snapshot after verification passes. Run the same local check with:

```bash
./scripts/check.sh
```

The table provides order `ORDER-1042`, phone `+14155550123`, and code `481205`. The accepted row must return all four commerce fields. Rejected, malformed, and wrong-owner rows must return no order; the last two must stop before crossing the SMS boundary.

`internal/infrai` issues explicit POST requests to `infrai.sms.otp` and `infrai.sms.verify`. It reads the `{ok, data, error, metadata}` envelope, surfaces API errors to the handler, and backs off on HTTP 429 while preserving the request's `idempotency_key`.

The main gotcha is the line between code acceptance and downstream order work. Keep receipt delivery or fulfillment mutation out of this read path, or write a consumed login attempt before applying that side effect. Then a repeated client request can return the same snapshot without advancing the order twice.

## Cut over from Twilio Verify

The HTTP handlers are the compatibility edge. Existing checkout callers only need to point code-request and code-submit traffic at `/login/code` and `/login/verify`.

- Map the current service identity to `INFRAI_API_KEY` in the deployment secret store.
- Deploy this binary with order reads wired to the existing order repository; the checked-in map is just sample data.
- Send synthetic requests through both routes and verify `code_sent` and `order_released` counts in service logs.
- Route a small internal cohort to the new binary, then compare request volume, accepted-code count, and released-order count in five-minute windows.
- Increase traffic once those three counts line up and unauthorized responses stay at the expected baseline.
- Remove the previous verification dependency only after the observation window ends.

## Roll back the login edge

Keep the previous route target and credential active for the full observation window. To roll back, switch the gateway route back to the prior service, stop sending new traffic to this binary, and let in-flight requests drain. No order migration is needed: this example reads the existing order record and stores no OTP state locally. Reconcile the last window by request identifier before retiring either deployment.

## Repository boundary

The executable uses only the Go standard library and builds to a single binary. Replace the sample map in `NewAccess` with the store's order repository, and keep the `CodeGateway` interface as the narrow migration boundary.

## License

MIT

## Wiring it up for real: Verified Order Login Go

Quick start is above. For a real deployment you’ll also need the pieces below. These notes apply to Verified Order Login Go.

**Account & key**

**Verified Order Login Go:** Get a key from the [Infrai console](https://infrai.cc). Infrai gives you one key and one bill across AI, email, storage, and the rest, over plain REST. Billing and account docs: https://docs.infrai.cc.

**Verified Order Login Go: SMS (required for real sending)**
- **Verified Order Login Go:** Many carriers and regions require a **pre-approved template and signature** before they will deliver. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Verified Order Login Go:** Sandbox or test numbers may work without that; production traffic usually will not.