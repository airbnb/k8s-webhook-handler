FROM golang:1.12 AS build-env
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod tidy
ENV GOLANGCI_LINT_VERSION=v1.22.0
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin $GOLANGCI_LINT_VERSION

COPY . .
RUN golangci-lint run --timeout 30m
RUN go build ./... && go test ./... && go install ./...

FROM alpine:3.6
RUN apk add --update ca-certificates git openssh-client \
  && addgroup -g 1000 user \
  && adduser -u 1000 -D user -G user \
  && ssh-keyscan github.com > /etc/ssh/ssh_known_hosts
USER user
COPY --from=0 /go/bin/* /usr/bin/
ENTRYPOINT [ "webhook" ]
