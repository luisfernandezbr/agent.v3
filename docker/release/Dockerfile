FROM golang:alpine
RUN apk add make git
RUN apk add openrc --no-cache;
WORKDIR $GOPATH/src/github.com/pinpt/agent

# Copy source code
COPY . .

# Build the actual binaries
ARG VERSION=
RUN go run ./cmd/agent-dev build --platform linux --skip-archives --version ${VERSION}
RUN mkdir /tmp/agent && cp -R dist/bin/linux-amd64/ /tmp/agent/

FROM alpine
RUN apk update && apk upgrade && apk add --no-cache bash git openssh ca-certificates gnupg
COPY --from=0 /tmp/agent/linux-amd64 /bin
RUN mv /bin/pinpoint-agent /bin/pinpoint-agent
WORKDIR /
ENTRYPOINT ["pinpoint-agent"]