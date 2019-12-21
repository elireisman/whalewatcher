FROM golang:1.13.4 AS build

WORKDIR /go/src/github.com/elireisman/whalewatcher

ADD . .

RUN make

FROM alpine:latest AS release

RUN apk --update add ca-certificates && rm -f /var/cache/apk/*

COPY --from=build /go/src/github.com/elireisman/whalewatcher/bin/* /bin/

EXPOSE 8080

CMD ["/bin/whalewatcher"]
