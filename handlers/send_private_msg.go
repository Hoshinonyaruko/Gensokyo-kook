package handlers

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/config"
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

	//如果获取不到 就用group_id获取信息类型
	if msgType == "" {
		msgType = GetMessageTypeByGroupid(config.GetAppIDStr(), message.Params.GroupID)
	}
	//如果获取不到 就用user_id获取信息类型
	if msgType == "" {
		msgType = GetMessageTypeByUserid(config.GetAppIDStr(), message.Params.UserID)
	}
	//新增 内存获取不到从数据库获取
	if msgType == "" {
		msgType = GetMessageTypeByUseridV2(message.Params.UserID)
	}
	if msgType == "" {
		msgType = GetMessageTypeByGroupidV2(message.Params.GroupID)
	}
	var idInt64 int64
	var err error

	if message.Params.UserID != "" {
		idInt64, err = ConvertToInt64(message.Params.UserID)
	} else if message.Params.GroupID != "" {
		idInt64, err = ConvertToInt64(message.Params.GroupID)
	}

	//设置递归 对直接向gsk发送action时有效果
	if msgType == "" {
		messageCopy := message
		if err != nil {
			mylog.Printf("错误：无法转换 ID %v\n", err)
		} else {
			// 递归3次
			echo.AddMapping(idInt64, 4)
			// 递归调用handleSendPrivateMsg，使用设置的消息类型
			echo.AddMsgType(config.GetAppIDStr(), idInt64, "group_private")
			HandleSendPrivateMsg(client, Token, BaseUrl, messageCopy)
		}
	}

	switch msgType {
	case "guild_private":
		//当收到发私信调用 并且来源是频道
		retmsg, _ = HandleSendGuildChannelPrivateMsg(client, Token, BaseUrl, message, nil, nil)
	default:
		mylog.Printf("Unknown message type: %s", msgType)
	}
	//重置递归类型
	if echo.GetMapping(idInt64) <= 0 {
		echo.AddMsgType(config.GetAppIDStr(), idInt64, "")
	}
	echo.AddMapping(idInt64, echo.GetMapping(idInt64)-1)

	//递归3次枚举类型
	if echo.GetMapping(idInt64) > 0 {
		tryMessageTypes := []string{"group", "guild", "guild_private"}
		messageCopy := message // 创建message的副本
		echo.AddMsgType(config.GetAppIDStr(), idInt64, tryMessageTypes[echo.GetMapping(idInt64)-1])
		delay := config.GetSendDelay()
		time.Sleep(time.Duration(delay) * time.Millisecond)
		HandleSendPrivateMsg(client, Token, BaseUrl, messageCopy)
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
