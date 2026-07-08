# ---- build stage ----
FROM golang:1.26-bookworm AS build
WORKDIR /src

# deps first for layer caching (lockfile-pinned, reproducible)
COPY go.mod go.sum ./
RUN go mod download

# static binary (cedar-go and pgx are pure Go → CGO can be off)
COPY . .
# normalize policy perms (source files may be 0600) so the non-root runtime user can read them
RUN chmod -R a+rX policies
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/authz-service ./cmd/authz-service

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/authz-service /app/authz-service
COPY --from=build /src/policies /app/policies
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/authz-service"]
