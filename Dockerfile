FROM uhub.service.ucloud.cn/bluecity/golang:1.15.0-alpine as builder

LABEL maintainer=laokiea@163.com

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

WORKDIR /app

ENV SMEM_VERSION=1.4

RUN set -ex; \
    sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk update \
    && apk upgrade \
    && apk add util-linux \
    && apk add python2 \
    && wget https://www.selenic.com/smem/download/smem-${SMEM_VERSION}.tar.gz \
    && tar -zvxf smem-${SMEM_VERSION}.tar.gz \
    && cp ./smem-${SMEM_VERSION}/smem /usr/bin

COPY --from=builder /gexporter/gexpoter_main ./

EXPOSE 80

CMD ["./gexpoter_main"]