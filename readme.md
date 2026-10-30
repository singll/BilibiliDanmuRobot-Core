# BilibiliDanmuRobot-Core

BilibiliDanmuRobot 的 Go 核心模块，维护于 [Hello-Moeka/BilibiliDanmuRobot-Core](https://github.com/Hello-Moeka/BilibiliDanmuRobot-Core)。它负责直播间 WebSocket 通信、弹幕事件解析、消息处理和机器人业务逻辑，不是独立 GUI 程序。

本仓库基于 [xbclub/BilibiliDanmuRobot-Core](https://github.com/xbclub/BilibiliDanmuRobot-Core) 继续维护，并作为桌面应用 [BilibiliDanmuRobot](https://github.com/Hello-Moeka/BilibiliDanmuRobot) 的 Go module 依赖。

## 主要职责

- 获取直播间连接信息并建立弹幕 WebSocket 连接。
- 解码弹幕、礼物、舰长、醒目留言等直播事件，分派给机器人处理器。
- 提供欢迎、礼物感谢、盲盒统计、关键词回复、定时弹幕、抽奖和 AI 回复等业务模块。
- 将解析后的礼物事件送入答谢队列与盲盒统计流程。

盲盒查询支持 `10月盲盒` / `十月盲盒`、`本月盲盒`、`今日盲盒` / `今天盲盒`、`本周盲盒`，以及昨日、前日、上周、上月等说法；`盲盒统计` 汇总全部已记录历史。按北京时间的自然日、周一开始的自然周和自然月计算，普通观众查自己、主播查全直播间。日／周／月命令可追加“统计”，`盲盒` 默认查本月。查询复用现有日期字段，无须数据迁移。

## 礼物协议维护

当前维护分支保留旧版 `SEND_GIFT` 处理，并增加 `SEND_GIFT_V2` 支持。V2 消息中的 `data.pb` 经过 Base64 与 Protobuf 解码后，按礼物列表逐条转换为现有礼物结构，再交给礼物答谢与统计逻辑。WebSocket 连接也会使用服务端返回的 WSS 端口，并记录房间鉴权结果。

## 安装

```bash
go get github.com/Hello-Moeka/BilibiliDanmuRobot-Core@latest
```

机器人桌面端会在自己的 `go.mod` 中固定 Core 版本。请优先使用已发布的稳定 tag；开发中的协议变化可能需要配套更新桌面端。

## 开发与构建

### 环境

- Go 1.24 或更新版本

### 常用命令

```bash
go mod download
go test ./...
go build ./...
```

`go build ./...` 用于编译检查各 Go package；本模块不产生 GUI exe。桌面程序请到 [BilibiliDanmuRobot](https://github.com/Hello-Moeka/BilibiliDanmuRobot) 获取或构建。

## 目录结构

- `blivedm-go/`：WebSocket 客户端、数据包和消息结构。
- `handler/`：各类直播事件处理器。
- `logic/`：弹幕发送、感谢、统计和互动业务。
- `config/`、`entity/`、`http/`、`model/`、`svc/`：配置、数据实体、API 与服务上下文。

## 来源与致谢

本项目延续 [xbclub/BilibiliDanmuRobot-Core](https://github.com/xbclub/BilibiliDanmuRobot-Core) 的维护工作，并使用 [Akegarasu/blivedm-go](https://github.com/Akegarasu/blivedm-go) 的弹幕协议实现。保留上游提交历史及第三方来源信息。
