package handlers

import (
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
)

func init() {
	callapi.RegisterHandler("send_private_msg_async", HandleSendPrivateMsg)
}
