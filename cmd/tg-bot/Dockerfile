# ---------- STAGE 1: build ----------
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Сборка бинарника tg-bot
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/tg-bot ./cmd/tg-bot

# ---------- STAGE 2: runtime ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/tg-bot /app/tg-bot

# HTTP порт бота + gRPC порт (TGBOT_GRPC_ADDR)
EXPOSE 8081 50051

CMD ["/app/tg-bot"]
