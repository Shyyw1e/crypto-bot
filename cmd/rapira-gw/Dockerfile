# ---------- STAGE 1: build ----------
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Сборка rapira-gw
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/rapira-gw ./cmd/rapira-gw

# ---------- STAGE 2: runtime ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/rapira-gw /app/rapira-gw

# HTTP порт rapira-gw (health/metrics/на будущее)
EXPOSE 8082

CMD ["/app/rapira-gw"]
