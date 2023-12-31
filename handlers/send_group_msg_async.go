package handlers

import (
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
)

func init() {
	callapi.RegisterHandler("send_group_msg_async", HandleSendGroupMsg)
}
