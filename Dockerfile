FROM golang:1.9
RUN go get -u github.com/kardianos/govendor
WORKDIR /go/src/github.com/itskoko/k8s-ci-purger
COPY vendor/ vendor/
RUN govendor sync
COPY . .
RUN go test $(go list ./... | grep -v /vendor/) \
  && CGO_ENABLED=0 go build

FROM alpine:3.6
RUN apk add --update ca-certificates && adduser -D user
USER user
COPY --from=0 /go/src/github.com/itskoko/k8s-ci-purger/k8s-ci-purger /usr/bin/
ENTRYPOINT [ "k8s-ci-purger" ]
