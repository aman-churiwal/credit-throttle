# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o transaction-service .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/transaction-service .

EXPOSE 8080

CMD ["./transaction-service"]