FROM golang:1.24.5-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY db ./db
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o mercado-telegram

FROM alpine:3.22.1
WORKDIR /app
COPY --from=builder /app/mercado-telegram ./
RUN apk add --no-cache ca-certificates poppler-utils
CMD ["/app/mercado-telegram"]