package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/hoshinonyaruko/gensokyo-kook/url"
	event2 "github.com/idodo/golang-bot/kaihela/api/base/event"
	"github.com/idodo/golang-bot/kaihela/api/helper"
	"github.com/skip2/go-qrcode"
	"mvdan.cc/xurls" //xurls是一个从文本提取url的库 适用于多种场景
)

var BotID string

// 定义响应结构体
type ServerResponse struct {
	Data struct {
		MessageID int `json:"message_id"`
	} `json:"data"`
	Message string      `json:"message"`
	RetCode int         `json:"retcode"`
	Status  string      `json:"status"`
	Echo    interface{} `json:"echo"`
}

// 发送成功回执 todo 返回可互转的messageid 实现频道撤回api
func SendResponse(client callapi.Client, err error, message *callapi.ActionMessage) (string, error) {
	// 设置响应值
	response := ServerResponse{}
	response.Data.MessageID = 123 // todo 实现messageid转换
	response.Echo = message.Echo
	if err != nil {
		response.Message = err.Error() // 可选：在响应中添加错误消息
		//response.RetCode = -1          // 可以是任何非零值，表示出错
		//response.Status = "failed"
		response.RetCode = 0 //官方api审核异步的 审核中默认返回失败,但其实信息发送成功了
		response.Status = "ok"
	} else {
		response.Message = ""
		response.RetCode = 0
		response.Status = "ok"
	}

	// 转化为map并发送
	outputMap := structToMap(response)
	// 将map转换为JSON字符串
	jsonResponse, jsonErr := json.Marshal(outputMap)
	if jsonErr != nil {
		log.Printf("Error marshaling response to JSON: %v", jsonErr)
		return "", jsonErr
	}
	//发送给ws 客户端
	sendErr := client.SendMessage(outputMap)
	if sendErr != nil {
		mylog.Printf("Error sending message via client: %v", sendErr)
		return "", sendErr
	}

	mylog.Printf("发送成功回执: %+v", string(jsonResponse))
	return string(jsonResponse), nil
}

// 信息处理函数
func parseMessageContent(paramsMessage callapi.ParamsContent, message callapi.ActionMessage, client callapi.Client, Token string, BaseUrl string) (string, map[string][]string) {
	messageText := ""

	switch message := paramsMessage.Message.(type) {
	case string:
		mylog.Printf("params.message is a string\n")
		messageText = message
	case []interface{}:
		//多个映射组成的切片
		mylog.Printf("params.message is a slice (segment_type_koishi)\n")
		for _, segment := range message {
			segmentMap, ok := segment.(map[string]interface{})
			if !ok {
				continue
			}

			segmentType, ok := segmentMap["type"].(string)
			if !ok {
				continue
			}

			segmentContent := ""
			switch segmentType {
			case "text":
				segmentContent, _ = segmentMap["data"].(map[string]interface{})["text"].(string)
			case "image":
				fileContent, _ := segmentMap["data"].(map[string]interface{})["file"].(string)
				segmentContent = "[CQ:image,file=" + fileContent + "]"
			case "voice":
				fileContent, _ := segmentMap["data"].(map[string]interface{})["file"].(string)
				segmentContent = "[CQ:record,file=" + fileContent + "]"
			case "record":
				fileContent, _ := segmentMap["data"].(map[string]interface{})["file"].(string)
				segmentContent = "[CQ:record,file=" + fileContent + "]"
			case "at":
				qqNumber, _ := segmentMap["data"].(map[string]interface{})["qq"].(string)
				segmentContent = "[CQ:at,qq=" + qqNumber + "]"
			case "markdown":
				mdContent, _ := segmentMap["data"].(map[string]interface{})["data"].(string)
				segmentContent = "[CQ:markdown,data=" + mdContent + "]"
			}

			messageText += segmentContent
		}
	case map[string]interface{}:
		//单个映射
		mylog.Printf("params.message is a map (segment_type_trss)\n")
		messageType, _ := message["type"].(string)
		switch messageType {
		case "text":
			messageText, _ = message["data"].(map[string]interface{})["text"].(string)
		case "image":
			fileContent, _ := message["data"].(map[string]interface{})["file"].(string)
			messageText = "[CQ:image,file=" + fileContent + "]"
		case "voice":
			fileContent, _ := message["data"].(map[string]interface{})["file"].(string)
			messageText = "[CQ:record,file=" + fileContent + "]"
		case "record":
			fileContent, _ := message["data"].(map[string]interface{})["file"].(string)
			messageText = "[CQ:record,file=" + fileContent + "]"
		case "at":
			qqNumber, _ := message["data"].(map[string]interface{})["qq"].(string)
			messageText = "[CQ:at,qq=" + qqNumber + "]"
		case "markdown":
			mdContent, _ := message["data"].(map[string]interface{})["data"].(string)
			messageText = "[CQ:markdown,data=" + mdContent + "]"
		}
	default:
		mylog.Println("Unsupported message format: params.message field is not a string, map or slice")
	}
	//处理at
	messageText = transformMessageTextAt(messageText)

	//mylog.Printf(messageText)

	// 正则表达式部分
	var localImagePattern *regexp.Regexp
	var localRecordPattern *regexp.Regexp
	if runtime.GOOS == "windows" {
		localImagePattern = regexp.MustCompile(`\[CQ:image,file=file:///([^\]]+?)\]`)
	} else {
		localImagePattern = regexp.MustCompile(`\[CQ:image,file=file://([^\]]+?)\]`)
	}
	if runtime.GOOS == "windows" {
		localRecordPattern = regexp.MustCompile(`\[CQ:record,file=file:///([^\]]+?)\]`)
	} else {
		localRecordPattern = regexp.MustCompile(`\[CQ:record,file=file://([^\]]+?)\]`)
	}
	httpUrlImagePattern := regexp.MustCompile(`\[CQ:image,file=http://(.+)\]`)
	httpsUrlImagePattern := regexp.MustCompile(`\[CQ:image,file=https://(.+)\]`)
	base64ImagePattern := regexp.MustCompile(`\[CQ:image,file=base64://(.+)\]`)
	base64RecordPattern := regexp.MustCompile(`\[CQ:record,file=base64://(.+)\]`)
	httpUrlRecordPattern := regexp.MustCompile(`\[CQ:record,file=http://(.+)\]`)
	httpsUrlRecordPattern := regexp.MustCompile(`\[CQ:record,file=https://(.+)\]`)
	mdPattern := regexp.MustCompile(`\[CQ:markdown,data=base64://(.+)\]`)

	patterns := []struct {
		key     string
		pattern *regexp.Regexp
	}{
		{"local_image", localImagePattern},
		{"url_image", httpUrlImagePattern},
		{"url_images", httpsUrlImagePattern},
		{"base64_image", base64ImagePattern},
		{"base64_record", base64RecordPattern},
		{"local_record", localRecordPattern},
		{"url_record", httpUrlRecordPattern},
		{"url_records", httpsUrlRecordPattern},
		{"markdown", mdPattern},
	}

	foundItems := make(map[string][]string)
	for _, pattern := range patterns {
		matches := pattern.pattern.FindAllStringSubmatch(messageText, -1)
		for _, match := range matches {
			if len(match) > 1 {
				foundItems[pattern.key] = append(foundItems[pattern.key], match[1])
			}
		}
		// 移动替换操作到这里，确保所有匹配都被处理后再进行替换
		messageText = pattern.pattern.ReplaceAllString(messageText, "")
	}
	//最后再处理Url
	messageText = transformMessageTextUrl(messageText, message, client, Token, BaseUrl)

	// for key, items := range foundItems {
	// 	fmt.Printf("Key: %s, Items: %v\n", key, items)
	// }
	return messageText, foundItems
}

func isIPAddress(address string) bool {
	return net.ParseIP(address) != nil
}

// at处理
func transformMessageTextAt(messageText string) string {

	// 去除所有[CQ:reply,id=数字]
	replyRE := regexp.MustCompile(`\[CQ:reply,id=\d+\]`)
	messageText = replyRE.ReplaceAllString(messageText, "")

	// 使用正则表达式来查找所有[CQ:at,qq=数字]的模式
	re := regexp.MustCompile(`\[CQ:at,qq=(\d+)\]`)
	messageText = re.ReplaceAllStringFunc(messageText, func(m string) string {
		submatches := re.FindStringSubmatch(m)
		if len(submatches) > 1 {
			realUserID, err := idmap.RetrieveRowByIDv2(submatches[1])
			if err != nil {
				// 如果出错，也替换成相应的格式，但使用原始QQ号
				mylog.Printf("Error retrieving user ID: %v", err)
				return "(met)" + submatches[1] + "(met)"
			}

			// 在这里检查 GetRemoveBotAtGroup 和 realUserID 的长度
			if config.GetRemoveBotAtGroup() && len(realUserID) == 32 {
				return ""
			}

			return "(met)" + realUserID + "(met)"
		}
		return m
	})
	return messageText
}

// 链接处理
func transformMessageTextUrl(messageText string, message callapi.ActionMessage, client callapi.Client, Token string, BaseUrl string) string {
	// 是否处理url
	if config.GetTransferUrl() {
		// 判断服务器地址是否是IP地址
		serverAddress := config.GetServer_dir()
		isIP := isIPAddress(serverAddress)
		VisualIP := config.GetVisibleIP()

		// 使用xurls来查找和替换所有的URL
		messageText = xurls.Relaxed.ReplaceAllStringFunc(messageText, func(originalURL string) string {
			// 当服务器地址是IP地址且GetVisibleIP为false时，替换URL为空
			if isIP && !VisualIP {
				return ""
			}

			// 如果启用了URL到QR码的转换
			if config.GetUrlToQrimage() {
				// 将URL转换为QR码的字节形式
				qrCodeGenerator, _ := qrcode.New(originalURL, qrcode.High)
				qrCodeGenerator.DisableBorder = true
				qrSize := config.GetQrSize()
				pngBytes, _ := qrCodeGenerator.PNG(qrSize)
				//pngBytes 二维码图片的字节数据
				base64Image := base64.StdEncoding.EncodeToString(pngBytes)
				picmsg := processActionMessageWithBase64PicReplace(base64Image, message)
				ret := callapi.CallAPIFromDict(client, Token, BaseUrl, picmsg)
				mylog.Printf("发送url转图片结果:%v", ret)
				// 从文本中去除原始URL
				return "" // 返回空字符串以去除URL
			}

			// 根据配置处理URL
			if config.GetLotusValue() {
				// 连接到另一个gensokyo-kook
				shortURL := url.GenerateShortURL(originalURL)
				return shortURL
			} else {
				// 自己是主节点
				shortURL := url.GenerateShortURL(originalURL)
				// 使用getBaseURL函数来获取baseUrl并与shortURL组合
				return url.GetBaseURL() + "/url/" + shortURL
			}
		})
	}
	return messageText
}

// processActionMessageWithBase64PicReplace 将原有的callapi.ActionMessage内容替换为一个base64图片
func processActionMessageWithBase64PicReplace(base64Image string, message callapi.ActionMessage) callapi.ActionMessage {
	newMessage := createCQImageMessage(base64Image)
	message.Params.Message = newMessage
	return message
}

// createCQImageMessage 从 base64 编码的图片创建 CQ 码格式的消息
func createCQImageMessage(base64Image string) string {
	return "[CQ:image,file=base64://" + base64Image + "]"
}

// 处理at和其他定形文到onebotv11格式(cq码)
func RevertTransformedText(data interface{}, msgtype string, Token string, BaseUrl string, vgid int64, vuid int64, whitenable bool) string {
	var msg *event2.MessageKMarkdownEvent
	var menumsg bool
	var messageText string
	switch v := data.(type) {
	case *event2.MessageKMarkdownEvent:
		msg = (*event2.MessageKMarkdownEvent)(v)
	default:
		return ""
	}
	menumsg = false
	//单独一个空格的信息的空格用户并不希望去掉
	if msg.Content == " " {
		menumsg = true
		messageText = " "
	}

	if !menumsg {
		//处理前 先去前后空
		messageText = strings.TrimSpace(msg.Content)
	}
	//mylog.Printf("1[%v]", messageText)

	// 使用正则表达式来查找所有(met)数字(met)的模式
	re := regexp.MustCompile(`\(met\)(\d+)\(met\)`)
	// 使用正则表达式来替换找到的模式为[CQ:at,qq=用户ID]
	messageText = re.ReplaceAllStringFunc(messageText, func(m string) string {
		submatches := re.FindStringSubmatch(m)
		if len(submatches) > 1 {
			userID := submatches[1]
			if userID == BotID {
				if config.GetRemoveAt() {
					return ""
				} else {
					return "[CQ:at,qq=" + BotID + "]"
				}
			}
			// 不是 BotID，进行正常映射
			userID64, err := idmap.StoreIDv2(userID)
			if err != nil {
				//如果储存失败(数据库损坏)返回原始值
				mylog.Printf("Error storing ID: %v", err)
				return "[CQ:at,qq=" + userID + "]"
			}
			// 类型转换
			userIDStr := strconv.FormatInt(userID64, 10)
			// 经过转换的cq码
			return "[CQ:at,qq=" + userIDStr + "]"
		}
		return m
	})

	//结构 <@!>空格/内容
	//如果移除了前部at,信息就会以空格开头,因为只移去了最前面的at,但at后紧跟随一个空格
	if config.GetRemoveAt() {
		if !menumsg {
			//再次去前后空
			messageText = strings.TrimSpace(messageText)
		}
	}
	//mylog.Printf("2[%v]", messageText)

	// 检查是否需要移除前缀
	if config.GetRemovePrefixValue() {
		// 移除消息内容中第一次出现的 "/"
		if idx := strings.Index(messageText, "/"); idx != -1 {
			messageText = messageText[:idx] + messageText[idx+1:]
		}
	}

	// 检查是否启用白名单模式
	if config.GetWhitePrefixMode() && whitenable {
		// 获取白名单反转标志
		whiteBypassRevers := config.GetWhiteBypassRevers()

		// 获取白名单例外群数组（现在返回 int64 数组）
		whiteBypass := config.GetWhiteBypass()
		bypass := false

		// 根据 whiteBypassRevers 的值来改变逻辑
		if whiteBypassRevers {
			// 如果反转白名单效果，只有在白名单数组中的 vgid 或 vuid 才生效
			bypass = true // 默认设置为 true，意味着需要白名单检查
			for _, id := range whiteBypass {
				if id == vgid || id == vuid {
					bypass = false // 如果在白名单数组中找到了 vgid 或 vuid，设置为 false
					break
				}
			}
		} else {
			// 常规逻辑：检查 vgid 是否在白名单例外数组中
			for _, id := range whiteBypass {
				if id == vgid || id == vuid {
					bypass = true
					break
				}
			}
		}

		// 如果vgid不在白名单例外数组中，则应用白名单过滤
		if !bypass {
			// 获取白名单数组
			whitePrefixes := config.GetWhitePrefixs()
			// 加锁以安全地读取 TemporaryCommands
			idmap.MutexT.Lock()
			temporaryCommands := make([]string, len(idmap.TemporaryCommands))
			copy(temporaryCommands, idmap.TemporaryCommands)
			idmap.MutexT.Unlock()

			// 合并白名单和临时指令
			allPrefixes := append(whitePrefixes, temporaryCommands...)
			// 默认设置为不匹配
			matched := false

			// 遍历白名单数组，检查是否有匹配项
			for _, prefix := range allPrefixes {
				if strings.HasPrefix(messageText, prefix) {
					// 找到匹配项，保留 messageText 并跳出循环
					matched = true
					break
				}
			}

			// 如果没有匹配项，则将 messageText 置为兜底回复 兜底回复可空
			if !matched {
				messageText = ""
				SendMessage(config.GetNoWhiteResponse(), data, msgtype, Token, BaseUrl)
			}
		}
	}
	//mylog.Printf("3[%v]", messageText)
	//检查是否启用黑名单模式
	if config.GetBlackPrefixMode() {
		// 获取黑名单数组
		blackPrefixes := config.GetBlackPrefixs()
		// 遍历黑名单数组，检查是否有匹配项
		for _, prefix := range blackPrefixes {
			if strings.HasPrefix(messageText, prefix) {
				// 找到匹配项，将 messageText 置为空并停止处理
				messageText = ""
				break
			}
		}
	}
	// 移除以 GetVisualkPrefixs 数组开头的文本
	visualkPrefixs := config.GetVisualkPrefixs()
	var matchedPrefix *config.VisualPrefixConfig
	var isSpecialType bool    // 用于标记是否为特殊类型
	var originalPrefix string // 存储原始前缀

	// 处理特殊类型前缀
	specialPrefixes := make(map[int]string)
	for i, vp := range visualkPrefixs {
		if strings.HasPrefix(vp.Prefix, "*") {
			specialPrefixes[i] = vp.Prefix                                // 保存原始前缀
			visualkPrefixs[i].Prefix = strings.TrimPrefix(vp.Prefix, "*") // 移除 '*'
		}
	}

	for i, vp := range visualkPrefixs {
		if strings.HasPrefix(messageText, vp.Prefix) {
			if _, ok := specialPrefixes[i]; ok {
				isSpecialType = true
				originalPrefix = specialPrefixes[i] // 恢复原始前缀
			}
			// 检查 messageText 的长度是否大于 prefix 的长度
			if len(messageText) > len(vp.Prefix) {
				// 移除找到的前缀 且messageText不为空格
				if messageText != " " {
					messageText = strings.TrimPrefix(messageText, vp.Prefix)
					messageText = strings.TrimSpace(messageText)
					matchedPrefix = &vp
				}
				break // 只移除第一个匹配的前缀
			}
		}
	}

	// 已经完成了移除前缀等操作,进行aliases替换
	aliases := config.GetAlias()
	messageText = processMessageText(messageText, aliases)
	//mylog.Printf("4[%v]", messageText)
	// 检查是否启用白名单模式
	if config.GetWhitePrefixMode() && matchedPrefix != nil {
		// 获取白名单反转标志
		whiteBypassRevers := config.GetWhiteBypassRevers()

		// 获取白名单例外群数组（现在返回 int64 数组）
		whiteBypass := config.GetWhiteBypass()
		bypass := false

		// 根据 whiteBypassRevers 的值来改变逻辑
		if whiteBypassRevers {
			// 如果反转白名单效果，只有在白名单数组中的 vgid 或 vuid 才生效
			bypass = true // 默认设置为 true，意味着需要白名单检查
			for _, id := range whiteBypass {
				if id == vgid || id == vuid {
					bypass = false // 如果在白名单数组中找到了 vgid 或 vuid，设置为 false
					break
				}
			}
		} else {
			// 常规逻辑：检查 vgid 是否在白名单例外数组中
			for _, id := range whiteBypass {
				if id == vgid || id == vuid {
					bypass = true
					break
				}
			}
		}

		// 如果vgid不在白名单例外数组中，则应用白名单过滤
		if !bypass {
			allPrefixes := matchedPrefix.WhiteList
			idmap.MutexT.Lock()
			temporaryCommands := make([]string, len(idmap.TemporaryCommands))
			copy(temporaryCommands, idmap.TemporaryCommands)
			idmap.MutexT.Unlock()

			// 合并虚拟前缀的白名单和临时指令
			allPrefixes = append(allPrefixes, temporaryCommands...)
			matched := false

			// 遍历白名单数组，检查是否有匹配项
			for _, prefix := range allPrefixes {
				if strings.HasPrefix(messageText, prefix) {
					matched = true
					break
				}
			}

			// 如果没有匹配项，则将 messageText 置为对应的兜底回复
			if !matched {
				messageText = ""
				SendMessage(matchedPrefix.NoWhiteResponse, data, msgtype, Token, BaseUrl)
			}
		}
	}

	// 在返回 messageText 时，根据 isSpecialType 判断是否需要添加原始前缀
	if isSpecialType && matchedPrefix != nil {
		//移除开头的*
		originalPrefix = strings.TrimPrefix(originalPrefix, "*")
		messageText = originalPrefix + messageText
	}
	//mylog.Printf("5[%v]", messageText)
	// 如果未启用白名单模式或没有匹配的虚拟前缀，执行默认逻辑

	// 处理图片附件 不用处理 kook不支持用户发富文本信息
	// for _, attachment := range msg.Attachments {
	// 	if strings.HasPrefix(attachment.ContentType, "image/") {
	// 		// 获取文件的后缀名
	// 		ext := filepath.Ext(attachment.FileName)
	// 		md5name := strings.TrimSuffix(attachment.FileName, ext)

	// 		// 检查 URL 是否已包含协议头
	// 		var url string
	// 		if strings.HasPrefix(attachment.URL, "http://") || strings.HasPrefix(attachment.URL, "https://") {
	// 			url = attachment.URL
	// 		} else {
	// 			url = "http://" + attachment.URL // 默认使用 http，也可以根据需要改为 https
	// 		}

	// 		imageCQ := "[CQ:image,file=" + md5name + ".image,subType=0,url=" + url + "]"
	// 		messageText += imageCQ
	// 	}
	// }
	//mylog.Printf("6[%v]", messageText)
	return messageText
}

// replaceFirstOccurrence 替换字符串中的第一个匹配项
func replaceFirstOccurrence(s, old, new string) string {
	if idx := strings.Index(s, old); idx != -1 {
		return s[:idx] + new + s[idx+len(old):]
	}
	return s
}

// processMessageText 处理消息文本
func processMessageText(messageText string, aliases []string) string {
	for i := 0; i < len(aliases); i += 2 {
		// 确保别名数组中有成对的元素
		if i+1 < len(aliases) {
			messageText = replaceFirstOccurrence(messageText, aliases[i], aliases[i+1])
		}
	}
	return messageText
}

// 将收到的data.content转换为message segment todo,群场景不支持受图片,频道场景的图片可以拼一下
func ConvertToSegmentedMessage(data interface{}) []map[string]interface{} {
	// 强制类型转换，获取Message结构
	var msg *event2.MessageKMarkdownEvent
	var menumsg bool
	switch v := data.(type) {
	case *event2.MessageKMarkdownEvent:
		msg = (*event2.MessageKMarkdownEvent)(v)
	default:
		return nil
	}
	menumsg = false
	//单独一个空格的信息的空格用户并不希望去掉
	if msg.Content == " " {
		menumsg = true
	}
	var messageSegments []map[string]interface{}

	//kook 用户发不了图文混合
	// 处理Attachments字段来构建图片消息
	// for _, attachment := range msg.Attachments {
	// 	imageFileMD5 := attachment.FileName
	// 	for _, ext := range []string{"{", "}", ".png", ".jpg", ".gif", "-"} {
	// 		imageFileMD5 = strings.ReplaceAll(imageFileMD5, ext, "")
	// 	}
	// 	imageSegment := map[string]interface{}{
	// 		"type": "image",
	// 		"data": map[string]interface{}{
	// 			"file":    imageFileMD5 + ".image",
	// 			"subType": "0",
	// 			"url":     attachment.URL,
	// 		},
	// 	}
	// 	messageSegments = append(messageSegments, imageSegment)

	// 	// 在msg.Content中替换旧的图片链接
	// 	//newImagePattern := "[CQ:image,file=" + attachment.URL + "]"
	// 	//msg.Content = msg.Content + newImagePattern
	// }

	// 使用正则表达式查找所有的[@数字]格式
	r := regexp.MustCompile(`<@!(\d+)>`)
	atMatches := r.FindAllStringSubmatch(msg.Content, -1)
	for _, match := range atMatches {
		userID := match[1]

		userID64, err := idmap.StoreIDv2(userID)
		if err != nil {
			// 如果存储失败，记录错误并继续使用原始 userID
			mylog.Printf("Error storing ID: %v", err)
		} else {
			// 类型转换成功，使用新的 userID
			userID = strconv.FormatInt(userID64, 10)
		}

		// 构建at部分的映射并加入到messageSegments
		atSegment := map[string]interface{}{
			"type": "at",
			"data": map[string]interface{}{
				"qq": userID,
			},
		}
		messageSegments = append(messageSegments, atSegment)

		// 从原始内容中移除at部分
		msg.Content = strings.Replace(msg.Content, match[0], "", 1)
	}
	//结构 <@!>空格/内容
	//如果移除了前部at,信息就会以空格开头,因为只移去了最前面的at,但at后紧跟随一个空格
	if config.GetRemoveAt() {
		//再次去前后空
		if !menumsg {
			msg.Content = strings.TrimSpace(msg.Content)
		}
	}

	// 检查是否需要移除前缀
	if config.GetRemovePrefixValue() {
		// 移除消息内容中第一次出现的 "/"
		if idx := strings.Index(msg.Content, "/"); idx != -1 {
			msg.Content = msg.Content[:idx] + msg.Content[idx+1:]
		}
	}
	// 如果还有其他内容，那么这些内容被视为文本部分
	if msg.Content != "" {
		textSegment := map[string]interface{}{
			"type": "text",
			"data": map[string]interface{}{
				"text": msg.Content,
			},
		}
		messageSegments = append(messageSegments, textSegment)
	}
	//排列
	messageSegments = sortMessageSegments(messageSegments)
	return messageSegments
}

// ConvertToInt64 尝试将 interface{} 类型的值转换为 int64 类型
func ConvertToInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		// 当无法处理该类型时返回错误
		return 0, fmt.Errorf("无法将类型 %T 转换为 int64", value)
	}
}

// 排列MessageSegments
func sortMessageSegments(segments []map[string]interface{}) []map[string]interface{} {
	var atSegments, textSegments, imageSegments []map[string]interface{}

	for _, segment := range segments {
		switch segment["type"] {
		case "at":
			atSegments = append(atSegments, segment)
		case "text":
			textSegments = append(textSegments, segment)
		case "image":
			imageSegments = append(imageSegments, segment)
		}
	}

	// 按照指定的顺序合并这些切片
	return append(append(atSegments, textSegments...), imageSegments...)
}

// SendMessage 发送消息根据不同的类型
func SendMessage(messageText string, data interface{}, messageType string, Token string, BaseUrl string) error {
	// 强制类型转换，获取Message结构
	var msg *event2.MessageKMarkdownEvent
	switch v := data.(type) {
	case *event2.MessageKMarkdownEvent:
		msg = (*event2.MessageKMarkdownEvent)(v)
	default:
		return nil
	}

	switch messageType {
	case "guild":
		// 处理公会消息
		api := helper.NewApiHelper("/v3/message/create", Token, BaseUrl, "", "")

		Data := map[string]string{
			"type":       "1",
			"channel_id": msg.TargetId,
			"content":    messageText,
		}
		DataByte, err := sonic.Marshal(Data)
		if err != nil {
			return err
		}
		resp, err := api.SetBody(DataByte).Post()
		mylog.Printf("sent post:%s", api.String())
		if err != nil {
			return err
		}
		mylog.Printf("resp:%s", string(resp))

	case "guild_private":
		// 处理私信
		api := helper.NewApiHelper("/v3/direct-message/create", Token, BaseUrl, "", "")
		Data := map[string]string{
			"type":      "1",
			"target_id": msg.Author.ID,
			"content":   messageText,
		}
		DataByte, err := sonic.Marshal(Data)
		if err != nil {
			return err
		}
		resp, err := api.SetBody(DataByte).Post()
		mylog.Printf("sent post:%s", api.String())
		if err != nil {
			return err
		}
		mylog.Printf("resp:%s", string(resp))

	default:
		return errors.New("未知的消息类型")
	}

	return nil
}

// 将map转化为json string
func ConvertMapToJSONString(m map[string]interface{}) (string, error) {
	// 使用 json.Marshal 将 map 转换为 JSON 字节切片
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		log.Printf("Error marshalling map to JSON: %v", err)
		return "", err
	}

	// 将字节切片转换为字符串
	jsonString := string(jsonBytes)
	return jsonString, nil
}

//todo kmarkdown
// func parseMDData(mdData []byte) (*dto.Markdown, *keyboard.MessageKeyboard, error) {
// 	// 定义一个用于解析 JSON 的临时结构体
// 	var temp struct {
// 		Markdown struct {
// 			CustomTemplateID *string               `json:"custom_template_id,omitempty"`
// 			Params           []*dto.MarkdownParams `json:"params,omitempty"`
// 			Content          string                `json:"content,omitempty"`
// 		} `json:"markdown,omitempty"`
// 		Keyboard struct {
// 			ID      string                   `json:"id,omitempty"`
// 			Content *keyboard.CustomKeyboard `json:"content,omitempty"`
// 		} `json:"keyboard,omitempty"`
// 		Rows []*keyboard.Row `json:"rows,omitempty"`
// 	}

// 	// 解析 JSON
// 	if err := json.Unmarshal(mdData, &temp); err != nil {
// 		return nil, nil, err
// 	}

// 	// 处理 Markdown
// 	var md *dto.Markdown
// 	if temp.Markdown.CustomTemplateID != nil {
// 		// 处理模板 Markdown
// 		md = &dto.Markdown{
// 			CustomTemplateID: *temp.Markdown.CustomTemplateID,
// 			Params:           temp.Markdown.Params,
// 			Content:          temp.Markdown.Content,
// 		}
// 	} else if temp.Markdown.Content != "" {
// 		// 处理自定义 Markdown
// 		md = &dto.Markdown{
// 			Content: temp.Markdown.Content,
// 		}
// 	}

// 	// 处理 Keyboard
// 	var kb *keyboard.MessageKeyboard
// 	if temp.Keyboard.Content != nil {
// 		// 处理嵌套在 Keyboard 中的 CustomKeyboard
// 		kb = &keyboard.MessageKeyboard{
// 			ID:      temp.Keyboard.ID,
// 			Content: temp.Keyboard.Content,
// 		}
// 	} else if len(temp.Rows) > 0 {
// 		// 处理顶层的 Rows
// 		kb = &keyboard.MessageKeyboard{
// 			Content: &keyboard.CustomKeyboard{Rows: temp.Rows},
// 		}
// 	}

// 	return md, kb, nil
// }
