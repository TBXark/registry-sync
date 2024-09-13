.PHONY: build
build:
	go build -o ./build/ ./...

.PHONY: buildLinuxX86
buildLinuxX86:
	GOOS=linux GOARCH=amd64 go build -o ./build/ ./...

.PHONY: buildImage
buildImage: buildLinuxX86
	docker buildx build --platform=linux/amd64 -t ghcr.io/tbxark/registry-sync:latest . --push

.PHONY: buildImage
buildCrossImage:
	docker buildx build --platform=linux/amd64,linux/arm64 -t ghcr.io/tbxark/registry-sync:latest -f Dockerfile-cross . --push