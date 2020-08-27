FROM golang as builder

WORKDIR /workspace

# Copy everything
COPY . .

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download


# Build the actual binaries
ARG BUILD=
ENV COMMITSHA=${BUILD}
RUN make linux
RUN mkdir /tmp/agent && cp -R dist/bin/linux-amd64/ /tmp/agent/

FROM alpine
RUN apk update && apk upgrade && apk add --no-cache bash git openssh ca-certificates gnupg
COPY --from=0 /tmp/agent/linux-amd64 /bin
RUN mv /bin/pinpoint-agent /bin/pinpoint-agent
WORKDIR /
ENTRYPOINT ["pinpoint-agent"]