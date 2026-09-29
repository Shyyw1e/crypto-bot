FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/abcex-gw ./cmd/abcex-gw

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/bin/abcex-gw /app/abcex-gw

EXPOSE 8084
CMD ["/app/abcex-gw"]
