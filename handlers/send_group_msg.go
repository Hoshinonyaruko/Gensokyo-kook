package handlers

import (
	"encoding/base64"
	"io"
	"net/http"
	"strconv"

	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/echo"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/images"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
)

func init() {
	callapi.RegisterHandler("send_group_msg", HandleSendGroupMsg)
	callapi.RegisterHandler("send_to_group", HandleSendGroupMsg)
}

func HandleSendGroupMsg(client callapi.Client, Token string, BaseUrl string, message callapi.ActionMessage) (string, error) {
	// 使用 message.Echo 作为key来获取消息类型
	var msgType string
	if echoStr, ok := message.Echo.(string); ok {
		// 当 message.Echo 是字符串类型时执行此块
		msgType = echo.GetMsgTypeByKey(echoStr)
	}

	if msgType == "" {
		msgType = GetMessageTypeByGroupidV2(message.Params.GroupID)
	}

	if msgType == "" {
		msgType = GetMessageTypeByUseridV2(message.Params.UserID)
	}

	//兜底防止死循环
	if msgType == "" {
		msgType = "guild"
	}

	mylog.Printf("send_group_msg获取到信息类型:%v", msgType)

	var retmsg string

	switch msgType {
	case "guild":
		retmsg, _ = HandleSendGuildChannelMsg(client, Token, BaseUrl, message)
	case "guild_private":
		retmsg, _ = HandleSendGuildChannelPrivateMsg(client, Token, BaseUrl, message, nil, nil)
	default:
		mylog.Printf("Unknown message type: %s", msgType)
	}

	return retmsg, nil
}

type Text struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type Element struct {
	Type string `json:"type"`
	Src  string `json:"src,omitempty"`
	Text *Text  `json:"text,omitempty"`
}

type Module struct {
	Type     string    `json:"type"`
	Elements []Element `json:"elements,omitempty"`
	Text     *Text     `json:"text,omitempty"`
}

type Card struct {
	Type    string   `json:"type"`
	Theme   string   `json:"theme"`
	Size    string   `json:"size"`
	Modules []Module `json:"modules"`
}

// 组合图片card
func generateKaiheilaMessage(foundItems map[string][]string, messageText string, Token string, BaseUrl string) *Card {
	if imageURLs, ok := foundItems["local_image"]; ok && len(imageURLs) > 0 {
		// 从本地路径读取图片
		imageURL, err := images.UploadImage(imageURLs[0], Token, BaseUrl)
		if err != nil {
			return &Card{
				Type:  "card",
				Theme: "secondary",
				Size:  "lg",
				Modules: []Module{
					{
						Type: "section",
						Elements: []Element{
							{
								Type: "plain-text",
								Src:  "错误: 图片文件不存在",
							},
						},
					},
				},
			}
		}
		// 创建card并返回，当作URL图片处理
		return &Card{
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
			},
		}
	} else if imageURLs, ok := foundItems["url_image"]; ok && len(imageURLs) > 0 {
		newpiclink := "http://" + imageURLs[0]
		//转换开黑啦img url
		newpiclink, _ = uploadImageAndGetNewLink(newpiclink, Token, BaseUrl)
		// 发链接图片
		return &Card{
			Type:  "card",
			Theme: "secondary",
			Size:  "lg",
			Modules: []Module{
				{
					Type: "container",
					Elements: []Element{
						{
							Type: "image",
							Src:  newpiclink,
						},
					},
				},
			},
		}
	} else if imageURLs, ok := foundItems["url_images"]; ok && len(imageURLs) > 0 {
		newpiclink := "https://" + imageURLs[0]
		//转换开黑啦img url
		newpiclink, _ = uploadImageAndGetNewLink(newpiclink, Token, BaseUrl)
		// 发链接图片
		return &Card{
			Type:  "card",
			Theme: "secondary",
			Size:  "lg",
			Modules: []Module{
				{
					Type: "container",
					Elements: []Element{
						{
							Type: "image",
							Src:  newpiclink,
						},
					},
				},
			},
		}
	} else if base64Image, ok := foundItems["base64_image"]; ok && len(base64Image) > 0 {
		// todo 适配base64图片
		//因为QQ群没有 form方式上传,所以在gensokyo-kook内置了图床,需公网,或以lotus方式连接位于公网的gensokyo-kook
		//要正确的开放对应的端口和设置正确的ip地址在config,这对于一般用户是有一些难度的
		// 解码base64图片数据

		// 将解码的图片数据转换回base64格式并上传
		imageURL, err := images.UploadImageBase64(base64Image[0], Token, BaseUrl)
		if err != nil {
			mylog.Printf("failed to upload base64 image: %v", err)
			return nil
		}
		// 发链接图片
		return &Card{
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
			},
		}
	} else {
		// 返回文本信息
		return &Card{
			Type:  "card",
			Theme: "secondary",
			Size:  "lg",
			Modules: []Module{
				{
					Type: "section",
					Elements: []Element{
						{
							Type: "plain-text",
							Src:  messageText,
						},
					},
				},
			},
		}
	}
}

// 通过user_id获取类型
func GetMessageTypeByUserid(appID string, userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}

	key := appID + "_" + userIDStr
	return echo.GetMsgTypeByKey(key)
}

// 通过user_id获取类型
func GetMessageTypeByUseridV2(userID interface{}) string {
	// 从appID和userID生成key
	var userIDStr string
	switch u := userID.(type) {
	case int:
		userIDStr = strconv.Itoa(u)
	case int64:
		userIDStr = strconv.FormatInt(u, 10)
	case float64:
		userIDStr = strconv.FormatFloat(u, 'f', 0, 64)
	case string:
		userIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}
	msgtype, _ := idmap.ReadConfigv2(userIDStr, "type")
	// if err != nil {
	// 	//mylog.Printf("GetMessageTypeByUseridV2失败:%v", err)
	// }
	return msgtype
}

// 通过group_id获取类型
func GetMessageTypeByGroupid(appID string, GroupID interface{}) string {
	// 从appID和userID生成key
	var GroupIDStr string
	switch u := GroupID.(type) {
	case int:
		GroupIDStr = strconv.Itoa(u)
	case int64:
		GroupIDStr = strconv.FormatInt(u, 10)
	case string:
		GroupIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}

	key := appID + "_" + GroupIDStr
	return echo.GetMsgTypeByKey(key)
}

// 通过group_id获取类型
func GetMessageTypeByGroupidV2(GroupID interface{}) string {
	// 从appID和userID生成key
	var GroupIDStr string
	switch u := GroupID.(type) {
	case int:
		GroupIDStr = strconv.Itoa(u)
	case int64:
		GroupIDStr = strconv.FormatInt(u, 10)
	case string:
		GroupIDStr = u
	default:
		// 可能需要处理其他类型或报错
		return ""
	}

	msgtype, _ := idmap.ReadConfigv2(GroupIDStr, "type")
	// if err != nil {
	// 	//mylog.Printf("GetMessageTypeByGroupidV2失败:%v", err)
	// }
	return msgtype
}

func downloadImage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func uploadImageAndGetNewLink(imageURL string, token string, baseUrl string) (string, error) {
	// 下载图片
	imageData, err := downloadImage(imageURL)
	if err != nil {
		return "", err
	}

	// 将图片数据转换为base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// 上传图片并获取新链接
	newURL, err := images.UploadImageBase64(base64Image, token, baseUrl)
	if err != nil {
		return "", err
	}

	return newURL, nil
}
