package handlers

import (
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
)

func init() {
	callapi.RegisterHandler("send_msg_async", HandleSendMsg)
}
