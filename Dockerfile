FROM golang:1.9
RUN go get -u github.com/kardianos/govendor
WORKDIR /go/src/github.com/itskoko/k8s-ci-purger
COPY vendor/ vendor/
RUN govendor sync
COPY . .
RUN go test $(go list ./... | grep -v /vendor/) \
  && CGO_ENABLED=0 go build

ENTRYPOINT [ "./k8s-ci-purger" ]
