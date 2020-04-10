FROM golang:alpine
RUN apk add make dep git
ENV GOPATH=/codefresh/volume/go
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

# Run unit tests
RUN CGO_ENABLED=0 go test -v ./... -timeout 1m
