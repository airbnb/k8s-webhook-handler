FROM golang:1.9
RUN go get -u github.com/kardianos/govendor
WORKDIR /go/src/github.com/itskoko/k8s-ci-purger
COPY vendor/ vendor/
RUN govendor sync
COPY . .
RUN go test $(go list ./... | grep -v /vendor/) \
  && CGO_ENABLED=0 go install ./...

FROM alpine:3.6
RUN apk add --update ca-certificates git openssh-client \
  && adduser -D user \
  && ssh-keyscan github.com > /etc/ssh/ssh_known_hosts
USER user
COPY --from=0 /go/bin/* /usr/bin/
ENTRYPOINT [ "webhook" ]
