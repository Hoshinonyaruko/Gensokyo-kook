package base

import "github.com/gookit/event"

type SessionInterface interface {
	//接收数据
	ReceiveData(data []byte)

	SendData(data []byte)

	//注册回调
	On(message string, handler event.Listener)
}
