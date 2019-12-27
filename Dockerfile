FROM golang:1.13.4 AS build

WORKDIR /go/src/github.com/elireisman/whalewatcher

ADD . .

RUN make

FROM alpine:latest AS release

RUN apk --update add ca-certificates && \
  rm -f /var/cache/apk/* && \
  rm -f /var/cache/apk/* && \
  mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2


COPY --from=build /go/src/github.com/elireisman/whalewatcher/bin/* /bin/

EXPOSE 4444

CMD ["/bin/whalewatcher"]
