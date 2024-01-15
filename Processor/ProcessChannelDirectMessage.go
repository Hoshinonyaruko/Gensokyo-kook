// 处理收到的信息事件
package Processor

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/echo"
	"github.com/hoshinonyaruko/gensokyo-kook/handlers"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/idodo/golang-bot/kaihela/api/base"
	"github.com/idodo/golang-bot/kaihela/api/base/event"
)

// ProcessChannelDirectMessage 处理频道私信消息 这里我们是被动收到
func (p *Processors) ProcessChannelDirectMessage(data *event.MessageKMarkdownEvent) error {
	// 打印data结构体
	//PrintStructWithFieldNames(data)

	var userid64 int64

	var err error
	userid64, err = strconv.ParseInt(data.Author.ID, 10, 64)
	if err != nil {
		mylog.Printf("Error ParseInt userid64 127: %v", err)
		return nil
	}

	//获取当前的s值 当前ws连接所收到的信息条数
	s := base.GetGlobalS()
	if !p.Settings.GlobalPrivateToChannel {
		// 把频道类型的私信转换成普通ob11的私信

		//转换appidstring
		AppIDString := strconv.FormatUint(p.Settings.AppID, 10)
		echostr := AppIDString + "_" + strconv.FormatInt(s, 10)

		//将真实id写入数据库,可取出ChannelID
		idmap.WriteConfigv2(data.Author.ID, "channel_id", data.TargetId)
		//转换message_id
		messageID64, err := idmap.StoreIDv2(data.MsgId)
		if err != nil {
			log.Fatalf("Error storing ID: %v", err)
		}
		messageID := int(messageID64)
		//转换at
		messageText := handlers.RevertTransformedText(data, "guild_private", p.Token, p.BaseUrl, userid64, userid64, config.GetWhiteEnable(3))
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}
		//框架内指令
		//p.HandleFrameworkCommand(messageText, data, "guild_private")
		// 如果在Array模式下, 则处理Message为Segment格式
		var segmentedMessages interface{} = messageText
		if config.GetArrayValue() {
			segmentedMessages = handlers.ConvertToSegmentedMessage(data)
		}
		var IsBindedUserId bool
		if config.GetHashIDValue() {
			IsBindedUserId = idmap.CheckValue(data.Author.ID, userid64)
		} else {
			IsBindedUserId = idmap.CheckValuev2(userid64)
		}

		privateMsg := OnebotPrivateMessage{
			RawMessage:  messageText,
			Message:     segmentedMessages,
			MessageID:   messageID,
			MessageType: "private",
			PostType:    "message",
			SelfID:      int64(p.BotID),
			UserID:      userid64,
			Sender: PrivateSender{
				Nickname: data.Author.Username,
				UserID:   userid64,
			},
			SubType: "friend",
			Time:    time.Now().Unix(),
		}
		//增强字段
		if !config.GetNativeOb11() {
			privateMsg.RealMessageType = "guild_private"
			privateMsg.IsBindedUserId = IsBindedUserId
			privateMsg.Avatar = data.Author.Avatar
		}
		// 根据条件判断是否添加Echo字段
		if config.GetTwoWayEcho() {
			privateMsg.Echo = echostr
		}

		echo.AddMsgType(AppIDString, s, "guild_private")
		//其实不需要用AppIDString,因为gensokyo-kook是单机器人框架

		echo.AddMsgType(AppIDString, userid64, "guild_private")
		//储存当前群或频道号的类型
		idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "guild_private")

		// 调试
		PrintStructWithFieldNames(privateMsg)

		// Convert OnebotGroupMessage to map and send
		privateMsgMap := structToMap(privateMsg)
		//上报信息到onebotv11应用端(正反ws)
		p.BroadcastMessageToAll(privateMsgMap)
	} else {
		if !p.Settings.GlobalChannelToGroup {
			//将频道私信作为普通频道信息

			//转换at
			messageText := handlers.RevertTransformedText(data, "guild_private", p.Token, p.BaseUrl, 10000, 10000, config.GetWhiteEnable(3)) //todo 这里未转换
			if messageText == "" {
				mylog.Printf("信息被自定义黑白名单拦截")
				return nil
			}
			//框架内指令
			//p.HandleFrameworkCommand(messageText, data, "guild_private")

			//映射str的userid到int
			userid64, err := idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Printf("Error storing ID: %v", err)
				return nil
			}
			//OnebotChannelMessage
			onebotMsg := OnebotChannelMessage{
				ChannelID:   data.TargetId,
				GuildID:     data.GuildID,
				Message:     messageText,
				RawMessage:  messageText,
				MessageID:   data.MsgId,
				MessageType: "guild",
				PostType:    "message",
				SelfID:      int64(p.BotID),
				UserID:      userid64,
				SelfTinyID:  "",
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
				Time:    time.Now().Unix(),
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
			idmap.WriteConfigv2(data.TargetId, "type", "guild_private")
			//储存当前群或频道号的类型
			idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "guild_private")
			//todo 完善频道类型信息转换

			//调试
			PrintStructWithFieldNames(onebotMsg)

			// 将 onebotMsg 结构体转换为 map[string]interface{}
			msgMap := structToMap(onebotMsg)
			//上报信息到onebotv11应用端(正反ws)
			p.BroadcastMessageToAll(msgMap)
		} else {
			//将频道信息转化为群信息(特殊需求情况下)

			//转换at
			messageText := handlers.RevertTransformedText(data, "guild_private", p.Token, p.BaseUrl, userid64, userid64, config.GetWhiteEnable(3))
			if messageText == "" {
				mylog.Printf("信息被自定义黑白名单拦截")
				return nil
			}
			//框架内指令
			//p.HandleFrameworkCommand(messageText, data, "guild_private")

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
			var IsBindedUserId bool
			if config.GetHashIDValue() {
				IsBindedUserId = idmap.CheckValue(data.Author.ID, userid64)
			} else {
				IsBindedUserId = idmap.CheckValuev2(userid64)
			}
			groupMsg := OnebotGroupMessage{
				RawMessage:  messageText,
				Message:     segmentedMessages,
				MessageID:   messageID,
				GroupID:     userid64,
				MessageType: "group",
				PostType:    "message",
				SelfID:      int64(p.BotID),
				UserID:      userid64,
				Sender: Sender{
					Nickname: data.Author.Username,
					UserID:   userid64,
					TinyID:   "",
					Card:     data.Author.Username,
					Sex:      "0",
					Age:      0,
					Area:     "",
					Level:    "0",
				},
				SubType: "normal",
				Time:    time.Now().Unix(),
			}
			//增强字段
			if !config.GetNativeOb11() {
				groupMsg.RealMessageType = "guild_private"
				groupMsg.IsBindedUserId = IsBindedUserId
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
			} else {
				groupMsg.Sender.Role = "member"
			}

			//储存当前群或频道号的类型
			idmap.WriteConfigv2(fmt.Sprint(userid64), "type", "guild_private")

			//调试
			PrintStructWithFieldNames(groupMsg)

			// Convert OnebotGroupMessage to map and send
			groupMsgMap := structToMap(groupMsg)
			//上报信息到onebotv11应用端(正反ws)
			p.BroadcastMessageToAll(groupMsgMap)
		}

	}
	return nil
}
