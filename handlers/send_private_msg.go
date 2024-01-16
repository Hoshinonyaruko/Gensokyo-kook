package handlers

import (
	"github.com/bytedance/sonic"
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/echo"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/idodo/golang-bot/kaihela/api/helper"
)

func init() {
	callapi.RegisterHandler("send_private_msg", HandleSendPrivateMsg)
}

func HandleSendPrivateMsg(client callapi.Client, Token string, BaseUrl string, message callapi.ActionMessage) (string, error) {
	// 使用 message.Echo 作为key来获取消息类型
	var msgType string
	var retmsg string
	if echoStr, ok := message.Echo.(string); ok {
		// 当 message.Echo 是字符串类型时执行此块
		msgType = echo.GetMsgTypeByKey(echoStr)
	}

	if msgType == "" {
		msgType = GetMessageTypeByUseridV2(message.Params.UserID)
	}

	if msgType == "" {
		msgType = GetMessageTypeByGroupidV2(message.Params.GroupID)
	}

	switch msgType {
	case "guild_private":
		//当收到发私信调用 并且来源是频道
		retmsg, _ = HandleSendGuildChannelPrivateMsg(client, Token, BaseUrl, message, nil, nil)
	default:
		mylog.Printf("Unknown message type: %s", msgType)
	}

	return retmsg, nil
}

// 处理频道私信 最后2个指针参数可空 代表使用userid倒推
func HandleSendGuildChannelPrivateMsg(client callapi.Client, Token string, BaseUrl string, message callapi.ActionMessage, optionalGuildID *string, optionalChannelID *string) (string, error) {
	params := message.Params
	messageText, foundItems := parseMessageContent(params, message, client, Token, BaseUrl)

	var UserID string
	var retmsg string

	UserID = message.Params.UserID.(string)
	if UserID == "" {
		UserID = message.Params.GroupID.(string)
	}

	mylog.Println("私聊信息messageText:", messageText)
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

		api := helper.NewApiHelper("/v3/direct-message/create", Token, BaseUrl, "", "")

		// 将Card实例放入一个切片中
		cards := []*Card{newMessage}

		// kook的卡片需要的参数是一个card构成的[]而多个card可以叠加
		echoDataByte, err := sonic.Marshal(cards)
		if err != nil {
			return "", err
		}
		// 构造请求数据映射
		data := map[string]string{
			"type":      "10",
			"target_id": UserID,
			"content":   string(echoDataByte),
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
		api := helper.NewApiHelper("/v3/direct-message/create", Token, BaseUrl, "", "")

		// 构造请求数据映射
		data := map[string]string{
			"type":      "1",
			"target_id": UserID,
			"content":   messageText,
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
		mylog.Printf("发频道私信文本信息resp:%s", string(resp))
		//发送成功回执
		retmsg, _ = SendResponse(client, err, &message)
	}

	// 遍历foundItems并发送每种信息
	for key, urls := range foundItems {
		for _, url := range urls {
			var singleItem = make(map[string][]string)
			singleItem[key] = []string{url} // 创建一个只包含单个 URL 的 singleItem

			reply := generateKaiheilaMessage(singleItem, "", Token, BaseUrl)

			api := helper.NewApiHelper("/v3/direct-message/create", Token, BaseUrl, "", "")
			// 将Card实例放入一个切片中
			cards := []*Card{reply}
			echoDataByte, err := sonic.Marshal(cards)
			if err != nil {
				return "", err
			}
			// 构造请求数据映射
			data := map[string]string{
				"type":      "10",
				"target_id": UserID,
				"content":   string(echoDataByte),
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
			mylog.Printf("发频道私信信息resp:%s", string(resp))
			// 发送成功回执
			retmsg, _ = SendResponse(client, err, &message)
		}
	}
	return retmsg, nil
}
