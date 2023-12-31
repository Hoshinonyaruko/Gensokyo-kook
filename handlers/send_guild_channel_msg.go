package handlers

import (
	"github.com/bytedance/sonic"
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/idodo/golang-bot/kaihela/api/helper"
)

func init() {
	callapi.RegisterHandler("send_guild_channel_msg", HandleSendGuildChannelMsg)
}

func HandleSendGuildChannelMsg(client callapi.Client, Token string, BaseUrl string, message callapi.ActionMessage) (string, error) {
	// 使用 message.Echo 作为key来获取消息类型
	var msgType string
	var retmsg string

	if msgType == "" {
		msgType = GetMessageTypeByGroupidV2(message.Params.GroupID)
	}

	if msgType == "" {
		msgType = GetMessageTypeByUseridV2(message.Params.UserID)
	}

	//当不转换频道信息时(不支持频道私聊)
	if msgType == "" {
		msgType = "guild"
	}
	switch msgType {
	//原生guild信息
	case "guild":
		params := message.Params
		var channelID string
		var err error
		messageText, foundItems := parseMessageContent(params, message, client, Token, BaseUrl)
		if config.GetOb11Int32() {
			channelID, err = idmap.RetrieveRowByIDv2(message.Params.GroupID.(string))
			if err != nil {
				mylog.Printf("GetOb11Int32还原id时出错 %v", err)
			}
		} else {
			channelID = message.Params.GroupID.(string)
		}

		mylog.Println("频道发信息messageText:", messageText)
		//mylog.Println("foundItems:", foundItems)
		var singleItem = make(map[string][]string)
		var imageType, imageUrl string
		imageCount := 0

		// 检查不同类型的图片并计算数量
		if imageURLs, ok := foundItems["local_image"]; ok && len(imageURLs) == 1 {
			imageType = "local_image"
			imageUrl = imageURLs[0]
			imageCount++
		} else if imageURLs, ok := foundItems["url_image"]; ok && len(imageURLs) == 1 {
			imageType = "url_image"
			imageUrl = imageURLs[0]
			imageCount++
		} else if imageURLs, ok := foundItems["url_images"]; ok && len(imageURLs) == 1 {
			imageType = "url_images"
			imageUrl = imageURLs[0]
			imageCount++
		} else if base64Images, ok := foundItems["base64_image"]; ok && len(base64Images) == 1 {
			imageType = "base64_image"
			imageUrl = base64Images[0]
			imageCount++
		}

		if imageCount == 1 && messageText != "" {
			//我想优化一下这里,让它优雅一点
			mylog.Printf("发图文混合信息-频道")
			// 创建包含单个图片的 singleItem
			singleItem[imageType] = []string{imageUrl}

			reply := generateKaiheilaMessage(singleItem, "", Token, BaseUrl)
			imageURL := FindImageUrlInReply(reply)
			// 创建包含文本和base64图像信息的消息

			newMessage := &Card{
				Type:  "card",
				Theme: "secondary",
				Size:  "lg",
				Modules: []Module{
					{
						Type: "container",
						Elements: []Element{
							{
								Type: "image",
								Src:  imageURL,
							},
						},
					},
					{
						Type: "section",
						Text: &Text{
							Type:    "kmarkdown",
							Content: messageText,
						},
					},
				},
			}

			api := helper.NewApiHelper("/v3/message/create", Token, BaseUrl, "", "")

			// 将Card实例放入一个切片中
			cards := []*Card{newMessage}

			// kook的卡片需要的参数是一个card构成的[]而多个card可以叠加
			echoDataByte, err := sonic.Marshal(cards)
			if err != nil {
				return "", err
			}
			// 构造请求数据映射
			data := map[string]string{
				"type":       "10",
				"channel_id": channelID,
				"content":    string(echoDataByte),
			}

			// 序列化整个请求数据映射为JSON
			requestDataByte, err := sonic.Marshal(data)
			if err != nil {
				return "", err
			}

			resp, err := api.SetBody(requestDataByte).Post()
			mylog.Printf("sent post:%s", api.String())
			if err != nil {
				return "", err
			}
			mylog.Printf("发频道信息resp:%s", string(resp))
			// 发送成功回执
			retmsg, _ = SendResponse(client, err, &message)
			delete(foundItems, imageType) // 从foundItems中删除已处理的图片项
			messageText = ""
		}

		// 优先发送文本信息
		if messageText != "" {
			api := helper.NewApiHelper("/v3/message/create", Token, BaseUrl, "", "")

			// 构造请求数据映射
			data := map[string]string{
				"type":       "1",
				"channel_id": channelID,
				"content":    messageText,
			}

			// 序列化整个请求数据映射为JSON
			requestDataByte, err := sonic.Marshal(data)
			if err != nil {
				return "", err
			}
			resp, err := api.SetBody(requestDataByte).Post()
			mylog.Printf("sent post:%s", api.String())
			if err != nil {
				return "", err
			}
			mylog.Printf("发频道文本信息resp:%s", string(resp))
			//发送成功回执
			retmsg, _ = SendResponse(client, err, &message)
		}

		// 遍历foundItems并发送每种信息
		for key, urls := range foundItems {
			for _, url := range urls {
				singleItem[key] = []string{url} // 创建一个只有一个 URL 的 singleItem

				reply := generateKaiheilaMessage(singleItem, "", Token, BaseUrl)

				api := helper.NewApiHelper("/v3/message/create", Token, BaseUrl, "", "")
				// 将Card实例放入一个切片中
				cards := []*Card{reply}
				// kook的卡片需要的参数是一个card构成的[]而多个card可以叠加
				echoDataByte, err := sonic.Marshal(cards)
				if err != nil {
					return "", err
				}
				// 构造请求数据映射
				data := map[string]string{
					"type":       "10",
					"channel_id": channelID,
					"content":    string(echoDataByte),
				}
				// 序列化整个请求数据映射为JSON
				requestDataByte, err := sonic.Marshal(data)
				if err != nil {
					return "", err
				}
				resp, err := api.SetBody(requestDataByte).Post()
				mylog.Printf("sent post:%s", api.String())
				if err != nil {
					return "", err
				}
				mylog.Printf("发频道信息resp:%s", string(resp))
				// 发送成功回执
				retmsg, _ = SendResponse(client, err, &message)
			}
		}
	//频道私信 此时直接取出
	case "guild_private":
		params := message.Params
		channelID := params.ChannelID
		guildID := params.GuildID
		var RChannelID string
		var err error
		// 使用RetrieveRowByIDv2还原真实的ChannelID
		RChannelID, err = idmap.RetrieveRowByIDv2(channelID)
		if err != nil {
			mylog.Printf("error retrieving real UserID: %v", err)
		}
		retmsg, _ = HandleSendGuildChannelPrivateMsg(client, Token, BaseUrl, message, &guildID, &RChannelID)
	default:
		mylog.Printf("2Unknown message type: %s", msgType)
	}
	return retmsg, nil
}

func FindImageUrlInReply(reply *Card) string {
	for _, module := range reply.Modules {
		for _, element := range module.Elements {
			if element.Type == "image" {
				return element.Src
			}
		}
	}
	return "" // 如果没有找到图片，返回空字符串
}
