# 内网穿透工具

## 说明
- server：公网的服务器ip
- client：内网主机

## 使用

server端, 默认本地为5200端口
```bash
./server -l 5200 -r 3333
```

client端
```bash
./client -l 8080 -r 3333 -h 公网IP地址
```


## more
[ngrok](https://github.com/inconshreveable/ngrok)