# ── Stage 1: builder ────────────────────────────────────────────────────────
FROM golang:1.26.2-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /gopherci ./cmd

# ── Stage 2: export (make docker-extract) ───────────────────────────────────
FROM scratch AS export
COPY --from=builder /gopherci /gopherci

# ── Stage 3: final image (make docker-build) ───────────────────────────────
FROM debian:bookworm-slim AS final

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /gopherci /usr/local/bin/gopherci

ENTRYPOINT ["gopherci"]
