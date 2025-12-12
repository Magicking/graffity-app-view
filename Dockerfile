FROM golang:alpine AS builder
LABEL maintainer="Sylvain Laurent <s@6120.eu>"
RUN apk add --no-cache wget curl git build-base
WORKDIR /go/src/app
RUN mkdir -p /go/src/app
COPY *.go *.mod *.sum /go/src/app
RUN go get -v ./... && \
    go build -v -o /go/bin/gtextview /go/src/app/*.go

FROM alpine:latest
RUN mkdir -p /go/bin/gtextview
COPY --from=builder /go/bin/gtextview /go/bin/app/gtextview

ENTRYPOINT ["/go/bin/app/gtextview"]

EXPOSE 8080
