FROM docker.io/golang:1.17.5 AS build-env
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod tidy
# ENV GOLANGCI_LINT_VERSION=v1.43.0
# RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
#  | sh -s -- -b $(go env GOPATH)/bin $GOLANGCI_LINT_VERSION

COPY . .
# RUN golangci-lint run --timeout 30m
RUN go build ./... && go test ./... && go install ./...

FROM scratch
COPY --from=0 /go/bin/* /usr/bin/
ENTRYPOINT [ "webhook" ]
