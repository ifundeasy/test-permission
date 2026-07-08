---
paths:
  - "internal/adapter/inbound/**"
---
# API design (net/http)
- REST/JSON over stdlib `net/http`. Endpoints today: `POST /authorize`, `GET /health`. Routes are
  registered on a `ServeMux` with Go 1.22+ method-based patterns (`"POST /authorize"`).
- The inbound adapter is a thin driving adapter: decode/validate the request, call the `Authorizer`
  port, encode the result. **No business logic, no SQL, no Cedar calls here.**
- Validate the body first; return `400` with `{ "error": ... }` for invalid JSON or missing
  `{ principal, action, resource }`.
- Error shape: `{ "error": string }`. Never return raw internal/driver error text to clients — map
  failures to a generic `500 { "error": "authorization failed" }` (log detail server-side only).
- Add new transport (e.g. gRPC) as a new inbound adapter behind the same `Authorizer` port, not by
  bolting logic onto the HTTP handler.
