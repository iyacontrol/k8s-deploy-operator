FROM golang:1.12 AS build-env
ADD ./  /go/src/github.com/test/hello
WORKDIR /go/src/github.com/test/hello
RUN CGO_ENABLED=0  go build -i -o hello

FROM alpine
COPY --from=build-env /go/src/github.com/test/hello/hello /usr/local/bin/hello
RUN chmod +x /usr/local/bin/hello
ENTRYPOINT ["hello"]