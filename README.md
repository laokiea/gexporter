# gexporter

## Require
* os: linux
* user: root
* smem,strace

## 指标
*  cpu使用率, 包括user,system,total
*  内存使用率，包括pss,rss,uss
*  cpu负载
*  cpu信息，包括物理核数，逻辑核数等
*  strace信息

## config
*  抓取间隔 -scrape-interval=15
*  监控最大进程数 -max-process-num=1000
*  数据暴露处理，支持直接expose和pushgateway，-exporter=expose|pushgateway
*  服务端口，-prom-http-port=80

## Usage：
1. Run
`git clone git@git.blued.cn:sashengpeng/gexporter.git && cd gexporter/run && go build -o gexporter_main && ./gexporter_main -max-process-num=1000 &`
或者
`go get laokiea/gexporter`
2. 接入监控
`https://bluecity.feishu.cn/docs/doccnb2wPsfGJQbjimU9oCkh49c`
示例
```
curl -X POST -H "Content-Type: application/json" --data-raw { \
    "team": "team-im", \
    "app": "base-project", \ 
    "scrape": true, \      
    "metric_path": "/metrics", \
    "metric_port": 80, \
    "address": [ \
        "10.9.86.167" \
    ] \
} http://webhook.svc.bks-dev.ourbluecity.com/monitor 
```


## Prometheus dashboard json example 
http://grafana.svc.bks-dev.ourbluecity.com/d/_-79c_4Mk/im-backend-test?editview=dashboard_json&orgId=2

## 效果
http://grafana.svc.bks.ourbluecity.com/d/66UORb4Gk/load?orgId=2&var-instance=All
