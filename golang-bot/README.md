# golang-bot

# Bot
本代码是[kaiheila](http://developer.kaiheila.cn/doc)机器人的golang示例sdk， 用户可以直接使用该代码或者参照该代码来构建自己的机器人。

本代码既可以当成一个包，来供系统调用。也可以当成一个独立的机器人来运行。在本代码中有一个重要的概念叫session。当websocket/webhook在线时，我们认为它和服务器保持了一个session,然后我们可以通过session来处理数据了。

## 代码说明
该包开发用的golang 1.19版本，其他依赖的包在go.mod中可以查看，主要有：
* 状态机：github.com/looplab/fsm
* 消息总线：github.com/gookit/event, 这里在原有的消息总线上做了一定的修改，主要是支持前缀模糊匹配和对事件名称数字开头的支持，修改后的repo为 github.com/idodo/event
* websocket：github.com/gorilla/websocket
* 退避重试：github.com/avast/retry-go/v4
* 定时任务：github.com/robfig/cron


### 代码使用

```golang
session := base.NewWebSocketSession(conf.Token, conf.BaseUrl, "./session.pid", "", 1)
// 注册接收frame事件回调，当session收到了正确的frame数据时，就会调用此方法
session.On(base.EventReceiveFrame, &handler.ReceiveFrameHandler{})

// 事件名支持通配符匹配，如下代表侦听群聊的所有消息
session.On("GROUP*", &handler.GroupEventHandler{})

// 事件名默认为 channel_type + _ + type组成， 如下代表侦听群聊的文字消息
session.On("GROUP_9", &handler.GroupTextEventHandler{Token: conf.Token, BaseUrl: conf.BaseUrl})
session.Start()


// 代码默认是以异步goroutine的方式处理收到的事件，如果需要同步可以在初始化session之后设置同步标识为true：
session.EventSyncHandle = true


// 通过webhook/websocket收到消息后，把数据传给session处理即可，session就会自动按上面注册的事件进行处理。
session.ReceiveData(data)

```

在回调中，我们通常会跟据服务端返回的消息，来做一些动作，我们统一封装了ApiClient:
```
client := helper.NewApiHelper("/test", ws.Token, ws.BaseUrl, "", "")
// get示例
client.SetQuery(map[string]string{"foo": "bar"})
resp, err := client.Get()
// post示例
data := []byte(`{"foo":"bar"}`)
client.SetBody( data )
resp, err := client.Post()
```

## kaiheila/api 作为module集成至其它服务内

```
go get -u github.com/idodo/golang-bot
````
参数上文或example, 直接使用`session.ReceiveData(data)`来处理数据即可。




## 独立机器人

本代码也可以作为一个独立的机器人来运行。

1. git clone git@github.com:idodo/golang-bot.git
2. 进入代码目录`cd golang-bot`，运行`go mod download`，如果下载依赖慢，请先设置国内github代理：`go env -w  GOPROXY=https://goproxy.cn,direct`
3. 打开[开发者中心](https://developer.kaiheila.cn/bot), 创建机器人，并更改为webhook/websocket模式。
4. 更改配置，将开发者中心的配置填入kaihela/example/conf/config.go文件中。

```bash
# cp kaihela/example/conf/config.go.example kaihela/example/conf/config.go
# 按照参数说明，修改config.go的配置, vim kaihela/example/conf/config.go
# 运行webhook机器人
go run kaihela/example/webhook/webhook.go

# 运行websocket机器人
go run kaihela/example/websock/websock.go
````
4. 在开发者后台，把机器人的地址填入后台。



