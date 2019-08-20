# would be good to set this by default for all commands, but other names are uncommon and will not be present on fs in practise
.PHONY: protobuf

# To run with a different pinpoint root
# make build-integrations PP_ROOT=~/.pinpoint/next-dev
PP_ROOT := ~/.pinpoint/next

protobuf:
	protoc -I rpcdef/proto/ rpcdef/proto/*.proto --go_out=plugins=grpc:rpcdef/proto/

build-integrations-local:
	go build -o ${PP_ROOT}/integrations/github ./integrations/github
	go build -o ${PP_ROOT}/integrations/jira-cloud ./integrations/jira-cloud 
	go build -o ${PP_ROOT}/integrations/jira-hosted ./integrations/jira-hosted
	go build -o ${PP_ROOT}/integrations/mock ./integrations/mock

build-prod-local:
	go build -tags prod -o dist/agent.next

build-macos:
	env GOOS=darwin go build -tags prod -o dist/macos/agent.next
	env GOOS=darwin go build -o dist/macos/integrations/github ./integrations/github
	env GOOS=darwin go build -o dist/macos/integrations/jira-cloud ./integrations/jira-cloud
	env GOOS=darwin go build -o dist/macos/integrations/jira-hosted ./integrations/jira-hosted
	env GOOS=darwin go build -o dist/macos/integrations/mock ./integrations/mock

build-linux:
	env GOOS=linux go build -tags prod -o dist/linux/agent.next
	env GOOS=linux go build -o dist/linux/integrations/github ./integrations/github
	env GOOS=linux go build -o dist/linux/integrations/jira-cloud ./integrations/jira-cloud
	env GOOS=linux go build -o dist/linux/integrations/jira-hosted ./integrations/jira-hosted
	env GOOS=linux go build -o dist/linux/integrations/mock ./integrations/mock

build-win:
	env GOOS=windows go build -tags prod -o dist/windows/agent-next.exe
	env GOOS=windows go build -o dist/windows/integrations/github.exe ./integrations/github
	env GOOS=windows go build -o dist/windows/integrations/jira-cloud.exe ./integrations/jira-cloud
	env GOOS=windows go build -o dist/windows/integrations/jira-hosted.exe ./integrations/jira-hosted
	env GOOS=windows go build -o dist/windows/integrations/mock.exe ./integrations/mock