# coald

煤矿场景下的设备监控 demo

## 主要功能

- 上报设备状态（csq 信号质量， btn 开关状态 等）
- 可通过服务端远程开关灯 （lighOn / lighOff）
- M2M，一台设备按钮按下后，远程过期另一台设备的灯

其中，通过 ubus 获取设备状态和控制设备

## 构建
```bash
GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags="-s -w" -o coald .
```
## 运行

```bash
# 此例为直接运行 golang 代码

# help
go run . --help

  -c    是否为控制端
  -ci string
        控制端的设备 thingId, 作为受控端时必需提供该值
  -e    是否为受控执行端
  -h string
        MQTT服务地址，域名或IP
  -i string
        当前设备的 thingId
  -p int
        MQTT 服务端口 (default 1883)
  -s string
        当前设备的接入密码

## 运行

go run . -h localhost -p 1883 -i test -s test -e -c -ci example

## 运行另一个
go run . -h localhost -p 1883 -i example -s example -e -c -ci test
```
