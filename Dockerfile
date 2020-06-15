FROM golang:1.14.4-alpine
ADD  . /go/src/github.com/googleinterns/knative-continuous-delivery
RUN  go install ./...

FROM alpine:latest
COPY --from=0 /go/bin/controller .
ENV  PORT 8080
CMD  ["./controller"]
