FROM golang:alpine
RUN apk add make dep git
RUN apk add openrc --no-cache;
WORKDIR $GOPATH/src/github.com/pinpt/agent

# Do not require rebuilding container if dependencies are the same
COPY Gopkg.toml .
COPY Gopkg.lock .
# The downloaded vendored packages are saved,
# but the vendor dir itself will be removed in next step
RUN dep ensure -v -vendor-only

# Copy source code
COPY . .

# Restore vendor dir from cache
RUN dep ensure -v -vendor-only

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