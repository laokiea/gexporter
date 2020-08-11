# gexporter

## 指标
*  cpu使用率, 包括user,system,total
*  内存使用率，包括pss,rss,uss
*  cpu负载
*  cpu信息，包括物理核数，逻辑核数等

## Usage：
cd run && go build -o gexporter_main && ./gexporter_main &

## metrics
curl -X GET http://127.0.0.1:80/metrics