package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/idodo/golang-bot/kaihela/api/helper"
)

func init() {
	callapi.RegisterHandler("get_group_list", GetGroupList)
}

// 定义全局变量来跟踪上次调用时间和当前页码
var (
	lastGuildsCallTime time.Time
	currentGuildPage   int
)

type Group struct {
	GroupCreateTime int32  `json:"group_create_time"`
	GroupID         int64  `json:"group_id"`
	GroupLevel      int32  `json:"group_level"`
	GroupMemo       string `json:"group_memo"`
	GroupName       string `json:"group_name"`
	MaxMemberCount  int32  `json:"max_member_count"`
	MemberCount     int32  `json:"member_count"`
}

type GroupList struct {
	Data    []Group     `json:"data"`
	Message string      `json:"message"`
	RetCode int         `json:"retcode"`
	Status  string      `json:"status"`
	Echo    interface{} `json:"echo"`
}

func GetGroupList(client callapi.Client, Token string, BaseUrl string, message callapi.ActionMessage) (string, error) {
	//群还不支持,这里取得是频道的,如果后期支持了群,那都请求,一起返回
	var groupList GroupList
	var err error
	// 初始化 groupList.Data 为一个空数组
	groupList.Data = []Group{}

	api := helper.NewApiHelper("/v3/guild/list", Token, BaseUrl, "", "")
	apilist := helper.NewApiHelper("/v3/channel/list", Token, BaseUrl, "", "")
	// 获取服务器列表
	guilds, err := FetchGuilds(api)
	if err != nil {
		mylog.Println("Error fetching guilds:", err)
		return "", nil
	}

	//填入获取服务器代码

	for _, guild := range guilds {

		groupID, _ := strconv.ParseInt(guild.ID, 10, 64)
		group := Group{
			GroupCreateTime: 0,
			GroupID:         groupID,
			GroupLevel:      0,
			GroupMemo:       guild.Topic,
			GroupName:       "*" + guild.Name,
			MaxMemberCount:  1000, // 确保这里也是 int32 类型
			MemberCount:     123,  // 将这里也转换为 int32 类型
		}
		groupList.Data = append(groupList.Data, group)

		// 根据服务器获取子频道
		channels, err := FetchChannels(apilist, guild.ID)
		if err != nil {
			mylog.Printf("Error fetching channels for guild %s: %v", guild.ID, err)
			continue
		}
		// 将channel信息转换为Group对象并添加到groups
		for _, channel := range channels {
			//转换ChannelID64
			ChannelID64, err := idmap.StoreIDv2(channel.ID)
			if err != nil {
				mylog.Printf("Error storing ID: %v", err)
			}
			channelGroup := Group{
				GroupCreateTime: 0, // 频道没有直接对应的创建时间字段
				GroupID:         ChannelID64,
				GroupLevel:      0,  // 频道没有直接对应的级别字段
				GroupMemo:       "", // 频道没有直接对应的描述字段
				GroupName:       channel.Name,
				MaxMemberCount:  0, // 频道没有直接对应的最大成员数字段
				MemberCount:     0, // 频道没有直接对应的成员数字段
			}
			groupList.Data = append(groupList.Data, channelGroup)
		}
	}

	groupList.Message = ""
	groupList.RetCode = 0
	groupList.Status = "ok"

	if message.Echo == "" {
		groupList.Echo = "0"
	} else {
		groupList.Echo = message.Echo
	}
	outputMap := structToMap(groupList)

	mylog.Printf("getGroupList(频道): %+v\n", outputMap)

	err = client.SendMessage(outputMap)
	if err != nil {
		mylog.Printf("error sending group info via wsclient: %v", err)
	}

	result, err := json.Marshal(groupList)
	if err != nil {
		mylog.Printf("Error marshaling data: %v", err)
		return "", nil
	}

	mylog.Printf("get_group_list: %s", result)
	return string(result), nil
}

// 定义全局变量来跟踪上次调用时间和当前页码
var (
	lastCallTime time.Time
	currentPage  int
)

// ChannelData 结构体用于存储频道信息
type ChannelData struct {
	ID   string
	Name string
	Type int
}

// FetchChannels 用于获取子频道列表
func FetchChannels(api *helper.ApiHelper, guildID string) ([]ChannelData, error) {
	// 构造请求参数
	params := map[string]string{
		"guild_id": guildID,
		"type":     "1", // 文字频道
	}

	api.SetQuery(params)
	//"/api/v3/channel/list"
	// 发起请求获取频道列表
	resp, err := api.Get()
	if err != nil {
		return nil, err
	}

	// 解析响应数据
	var channelListResponse struct {
		Code    int
		Message string
		Data    struct {
			Items []ChannelData
		}
	}
	err = json.Unmarshal(resp, &channelListResponse)
	if err != nil {
		return nil, err
	}

	return channelListResponse.Data.Items, nil
}

// FetchGuilds 用于获取当前用户加入的服务器列表
func FetchGuilds(api *helper.ApiHelper) ([]GuildData, error) {
	const pageSize = 10

	// 检查是否超过10分钟，如果是，则重置分页
	if time.Since(lastGuildsCallTime) > 10*time.Minute {
		currentGuildPage = 1 // 重置为第一页
	} else {
		currentGuildPage++ // 否则，递增页码
	}
	lastGuildsCallTime = time.Now() // 更新上次调用时间

	// 构造请求参数
	params := map[string]string{
		"page":      strconv.Itoa(currentGuildPage),
		"page_size": strconv.Itoa(pageSize),
		// "sort":    "id", // 如果需要特定的排序方式，可以添加这个参数
	}
	api.SetQuery(params)
	// 发起请求获取服务器列表
	resp, err := api.Get()
	if err != nil {
		return nil, err
	}

	// 解析响应数据
	var guildListResponse struct {
		Code    int
		Message string
		Data    struct {
			Items []GuildData
		}
	}
	err = json.Unmarshal(resp, &guildListResponse)
	if err != nil {
		return nil, err
	}

	return guildListResponse.Data.Items, nil
}
