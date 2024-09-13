FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o main main.go

FROM alpine:latest
COPY --from=builder /app/main /main
ENTRYPOINT ["/main"]
CMD ["--config", "/config/config.json"]