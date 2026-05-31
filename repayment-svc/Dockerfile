# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o repayment-service .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/repayment-service .

EXPOSE 8081

CMD ["./repayment-service"]