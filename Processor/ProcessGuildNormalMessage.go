// 处理收到的信息事件
package Processor

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/handlers"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/idodo/golang-bot/kaihela/api/base/event"
)

// ProcessGuildNormalMessage 处理频道常规消息
func (p *Processors) ProcessGuildNormalMessage(data *event.MessageKMarkdownEvent) error {
	if !p.Settings.GlobalChannelToGroup {
		// 将时间字符串转换为时间戳

		//转换at
		messageText := handlers.RevertTransformedText(data, "guild", p.Token, p.BaseUrl, 10000, 10000, config.GetWhiteEnable(2)) //这里未转换
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}
		var userid64 int64
		var err error
		//框架内指令
		//p.HandleFrameworkCommand(messageText, data, "guild")

		//映射str的userid到int
		userid64, err = strconv.ParseInt(data.Author.ID, 10, 64)
		if err != nil {
			mylog.Printf("Error ParseInt userid64 127: %v", err)
			return nil
		}

		// 如果在Array模式下, 则处理Message为Segment格式
		var segmentedMessages interface{} = messageText
		if config.GetArrayValue() {
			segmentedMessages = handlers.ConvertToSegmentedMessage(data)
		}
		// 处理onebot_channel_message逻辑
		onebotMsg := OnebotChannelMessage{
			ChannelID:   data.TargetId,
			GuildID:     data.GuildID,
			Message:     segmentedMessages,
			RawMessage:  messageText,
			MessageID:   data.MsgId,
			MessageType: "guild",
			PostType:    "message",
			SelfID:      int64(p.BotID),
			UserID:      userid64,
			SelfTinyID:  "0",
			Sender: Sender{
				Nickname: data.Author.Username,
				TinyID:   "0",
				UserID:   userid64,
				Card:     data.Author.Username,
				Sex:      "0",
				Age:      0,
				Area:     "0",
				Level:    "0",
			},
			SubType: "channel",
			Time:    data.MsgTimestamp,
			Avatar:  data.Author.Avatar,
		}

		// 获取MasterID数组
		masterIDs := config.GetMasterID()

		// 判断userid64是否在masterIDs数组里
		isMaster := false
		for _, id := range masterIDs {
			if strconv.FormatInt(userid64, 10) == id {
				isMaster = true
				break
			}
		}

		// 根据isMaster的值为groupMsg的Sender赋值role字段
		if isMaster {
			onebotMsg.Sender.Role = "owner"
		} else {
			onebotMsg.Sender.Role = "member"
		}

		//储存当前群或频道号的类型
		idmap.WriteConfigv2(data.TargetId, "type", "guild")

		//调试
		PrintStructWithFieldNames(onebotMsg)

		// 将 onebotMsg 结构体转换为 map[string]interface{}
		msgMap := structToMap(onebotMsg)

		//上报信息到onebotv11应用端(正反ws)
		p.BroadcastMessageToAll(msgMap)
	} else {
		// GlobalChannelToGroup为true时的处理逻辑
		//将频道转化为一个群

		var userid64 int64
		var ChannelID64 int64
		var err error
		userid64, err = strconv.ParseInt(data.Author.ID, 10, 64)
		if err != nil {
			mylog.Printf("Error ParseInt userid64 127: %v", err)
			return nil
		}
		if config.GetOb11Int32() {
			ChannelID64, err = idmap.StoreIDv2(data.TargetId)
			if err != nil {
				mylog.Printf("Error ParseInt ChannelID64 136: %v", err)
				return nil
			}
		} else {
			ChannelID64, err = strconv.ParseInt(data.TargetId, 10, 64)
			if err != nil {
				mylog.Printf("Error ParseInt ChannelID64 142: %v", err)
				return nil
			}
		}
		//储存原来的(获取群列表需要)
		idmap.WriteConfigv2(data.TargetId, "guild_id", data.GuildID)
		//转换at
		messageText := handlers.RevertTransformedText(data, "guild", p.Token, p.BaseUrl, ChannelID64, userid64, config.GetWhiteEnable(2))
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}
		//框架内指令
		//p.HandleFrameworkCommand(messageText, data, "guild")

		//映射str的messageID到int
		messageID64, err := idmap.StoreIDv2(data.MsgId)
		if err != nil {
			mylog.Printf("Error storing ID: %v", err)
			return nil
		}
		messageID := int(messageID64)
		// 如果在Array模式下, 则处理Message为Segment格式
		var segmentedMessages interface{} = messageText
		if config.GetArrayValue() {
			segmentedMessages = handlers.ConvertToSegmentedMessage(data)
		}
		var IsBindedUserId, IsBindedGroupId bool
		if config.GetHashIDValue() {
			IsBindedUserId = idmap.CheckValue(data.Author.ID, userid64)
			IsBindedGroupId = idmap.CheckValue(data.TargetId, ChannelID64)
		} else {
			IsBindedUserId = idmap.CheckValuev2(userid64)
			IsBindedGroupId = idmap.CheckValuev2(ChannelID64)
		}
		groupMsg := OnebotGroupMessage{
			RawMessage:  messageText,
			Message:     segmentedMessages,
			MessageID:   messageID,
			GroupID:     ChannelID64,
			MessageType: "group",
			PostType:    "message",
			SelfID:      int64(p.BotID),
			UserID:      userid64,
			Sender: Sender{
				Nickname: data.Author.Username,
				UserID:   userid64,
				Card:     data.Author.Username,
				Sex:      "0",
				Age:      0,
				Area:     "",
				Level:    "0",
			},
			SubType: "normal",
			Time:    time.Now().Unix(),
		}
		//增强配置
		if !config.GetNativeOb11() {
			groupMsg.RealMessageType = "guild"
			groupMsg.IsBindedUserId = IsBindedUserId
			groupMsg.IsBindedGroupId = IsBindedGroupId
			groupMsg.Avatar = data.Author.Avatar
		}

		// 获取MasterID数组
		masterIDs := config.GetMasterID()

		// 判断userid64是否在masterIDs数组里
		isMaster := false
		for _, id := range masterIDs {
			if strconv.FormatInt(userid64, 10) == id {
				isMaster = true
				break
			}
		}

		// 根据isMaster的值为groupMsg的Sender赋值role字段
		if isMaster {
			groupMsg.Sender.Role = "owner"
		}

		//储存当前群或频道号的类型
		idmap.WriteConfigv2(fmt.Sprint(ChannelID64), "type", "guild")

		//调试
		PrintStructWithFieldNames(groupMsg)

		// Convert OnebotGroupMessage to map and send
		groupMsgMap := structToMap(groupMsg)

		//上报信息到onebotv11应用端(正反ws)
		p.BroadcastMessageToAll(groupMsgMap)
	}

	return nil
}
