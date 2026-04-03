FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o petroapp .

FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/petroapp .

ENV PORT=8080
EXPOSE ${PORT}
ENTRYPOINT ["./petroapp"]
