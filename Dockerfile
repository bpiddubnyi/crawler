FROM golang:1.9-alpine
ADD . /go/src/github.com/bpiddubnyi/crawler
WORKDIR /go/src/github.com/bpiddubnyi/crawler/cmd/crawler
RUN go install
WORKDIR /go/src/github.com/bpiddubnyi/crawler/cmd/crawler-stat
RUN go install
ENTRYPOINT ["crawler"]
