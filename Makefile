# would be good to set this by default for all commands, but other names are uncommon and will not be present on fs in practise
.PHONY: protobuf

COMMITSHA ?= $(shell git rev-parse HEAD | cut -c1-8)

clean:
	@rm -rf logs dist

docker-dev:
	docker run --rm -it -v $(GOPATH)/src/github.com/pinpt/agent.next:/go/src/github.com/pinpt/agent.next $(shell docker build -q . -f docker/dev/Dockerfile)

dependencies:
	@rm -rf .vendor-new
	@dep ensure -v -vendor-only

proto:
	protoc -I rpcdef/proto/ rpcdef/proto/*.proto --go_out=plugins=grpc:rpcdef/proto/

build:
	go run ./cmd/agent-dev build --skip-archives

macos:
	go run ./cmd/agent-dev build --platform macos --skip-archives

osx: macos
darwin: macos

linux:
	go run ./cmd/agent-dev build --platform linux --skip-archives

windows:
	go run ./cmd/agent-dev build --platform windows --skip-archives

.PHONY: docker
docker:
	@docker build --build-arg BUILD=$(COMMITSHA) -t pinpt/agent .
