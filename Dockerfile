FROM golang:alpine
RUN apk add make dep git
WORKDIR $GOPATH/src/github.com/pinpt/agent.next

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

# Run unit tests
# RUN CGO_ENABLED=0 go test -v

# Build the actual binaries
ARG BUILD=
ENV PP_AGENT_VERSION=${BUILD}
RUN echo PP_AGENT_VERSION=$PP_AGENT_VERSION
RUN make linux
RUN mkdir /tmp/agent && cp -R dist/linux/ /tmp/agent/

FROM alpine
RUN apk update && apk upgrade && apk add --no-cache bash git openssh ca-certificates
COPY --from=0 /tmp/agent/linux /bin
RUN mv /bin/agent.next /bin/pinpoint-agent
WORKDIR /
ENTRYPOINT ["pinpoint-agent"]