FROM golang:1.12 AS build-env
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod tidy
COPY . .
RUN go build ./... && go install ./...

FROM alpine:3.6
RUN apk add --update ca-certificates git openssh-client \
  && adduser -D user \
  && ssh-keyscan github.com > /etc/ssh/ssh_known_hosts
USER user
COPY --from=0 /go/bin/* /usr/bin/
ENTRYPOINT [ "webhook" ]
