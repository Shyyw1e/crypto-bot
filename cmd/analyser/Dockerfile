# ---------- STAGE 1: build ----------
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Сначала зависимости — чтобы кешировалось
COPY go.mod go.sum ./
RUN go mod download

# Потом весь код
COPY . .

# Сборка бинарника analyser
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/analyser ./cmd/analyser

# ---------- STAGE 2: runtime ----------
FROM alpine:3.20

# Нужны сертификаты для HTTPS (Rapira, Telegram и т.п.)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Копируем бинарник из builder-стейджа
COPY --from=builder /app/bin/analyser /app/analyser

# (опционально) Копируем .env, если хочешь бандлить прямо в образ
# Обычно лучше монтировать/передавать извне, поэтому закомментировано:
# COPY .env /app/.env

# HTTP + gRPC порты analyser’а
EXPOSE 8080 50052

# Запуск сервиса
CMD ["/app/analyser"]
