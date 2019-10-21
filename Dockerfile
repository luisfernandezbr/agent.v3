FROM golang:alpine
RUN apk add make dep git
WORKDIR $GOPATH/src/github.com/pinpt/agent.next
COPY . .
RUN make dependencies
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