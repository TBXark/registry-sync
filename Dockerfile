FROM alpine:latest
COPY /build/registry-sync /main
ENTRYPOINT ["/main"]
CMD ["--config", "/config/config.json"]