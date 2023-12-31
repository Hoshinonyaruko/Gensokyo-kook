package handlers

import (
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
)

func init() {
	callapi.RegisterHandler("get_group_ban", SetGroupBan)
}

func SetGroupBan(client callapi.Client, Token string, BaseUrl string, message callapi.ActionMessage) (string, error) {

	// 从message中获取group_id和UserID
	groupID := message.Params.GroupID.(string)
	//receivedUserID := message.Params.UserID.(string)
	//读取ini 通过ChannelID取回之前储存的guild_id
	// guildID, err := idmap.ReadConfigv2(groupID, "guild_id")
	// if err != nil {
	// 	mylog.Printf("Error reading config: %v", err)
	// 	return "", nil
	// }
	// // 根据UserID读取真实的userid
	// realUserID, err := idmap.RetrieveRowByIDv2(receivedUserID)
	// if err != nil {
	// 	mylog.Printf("Error reading real userID: %v", err)
	// 	return "", nil
	// }

	// 读取消息类型
	msgType, err := idmap.ReadConfigv2(groupID, "type")
	if err != nil {
		mylog.Printf("Error reading config for message type: %v", err)
		return "", nil
	}

	// 根据消息类型进行操作
	switch msgType {
	case "group":
		mylog.Printf("setGroupBan(频道): 目前暂未开放该能力")
		return "", nil
	case "private":
		mylog.Printf("setGroupBan(频道): 目前暂未适配私聊虚拟群场景的禁言能力")
		return "", nil
	case "guild":
		mylog.Printf("setGroupBan(频道): 目前暂未适配kook的禁言能力")
		return "", nil
	}
	return "", nil
}
