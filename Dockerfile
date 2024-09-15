FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN make build

FROM alpine:latest
COPY --from=builder /app/build/registry-sync /main
ENTRYPOINT ["/main"]
CMD ["--config", "/config/config.json"]