FROM uhub.service.ucloud.cn/bluecity/golang:1.15.0-alpine as builder

WORKDIR /gexporter

COPY ./ ./

RUN set -x; \
    mkdir /gopath \
    && unset GOPATH \
    && go env -w GOPATH=/gopath \
    && go env -w GO111MODULE=on \
    && go env -w GOPROXY=https://goproxy.cn,direct \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s" -o gexpoter_main run/run.go

FROM uhub.service.ucloud.cn/bluecity/alpine:3.12

COPY --from=builder /gexporter/gexpoter_main ./

EXPOSE 80

CMD ["./gexpoter_main"]