# coald

煤矿场景下的设备监控 demo

主要功能

- 上报设备状态（csq 信号质量， btn 开关状态 等）
- 可通过服务端远程开关灯 （lighOn / lighOff）
- M2M，一台设备按钮按下后，远程过期另一台设备的灯

其中，通过 ubus 获取设备状态和控制设备

