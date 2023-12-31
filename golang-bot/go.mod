module github.com/idodo/golang-bot

go 1.19

require (
	github.com/avast/retry-go/v4 v4.3.3
	github.com/bytedance/sonic v1.8.1
	github.com/gookit/event v1.0.6
	github.com/gorilla/websocket v1.5.0
	github.com/looplab/fsm v1.0.1
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.9.0
)

replace github.com/gookit/event v1.0.6 => github.com/idodo/event v1.0.1

require (
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
)
