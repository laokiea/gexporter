# gexporter

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

## Usage：
cd run && go build -o gexporter_main && ./gexporter_main -max-process-num=1000 &

## metrics
curl -X GET http://127.0.0.1:80/metrics