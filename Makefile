# actuall all targets are not creating the file with a name, but the names are not common, so won't be a problem in practise
.PHONY: protobuf

# To run with a different pinpoint root
# make build-integrations PP_ROOT=~/.pinpoint/next-dev
PP_ROOT := ~/.pinpoint/next

protobuf:
	protoc -I rpcdef/proto/ rpcdef/proto/*.proto --go_out=plugins=grpc:rpcdef/proto/

build-integrations:
	go build -o ${PP_ROOT}/integrations/github github.com/pinpt/agent.next/integrations/github
	go build -o ${PP_ROOT}/integrations/jira-cloud github.com/pinpt/agent.next/integrations/jira-cloud 
	go build -o ${PP_ROOT}/integrations/jira-hosted github.com/pinpt/agent.next/integrations/jira-hosted
	go build -o ${PP_ROOT}/integrations/mock github.com/pinpt/agent.next/integrations/mock

build-prod:
	go build -tags prod -o dist/agent.next