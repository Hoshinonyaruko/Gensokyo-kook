package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/hoshinonyaruko/gensokyo-kook/callapi"
	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
)

type LoginInfoResponse struct {
	Data    LoginInfoData `json:"data"`
	Message string        `json:"message"`
	RetCode int           `json:"retcode"`
	Status  string        `json:"status"`
	Echo    interface{}   `json:"echo"`
}

type LoginInfoData struct {
	Nickname string `json:"nickname"`
	UserID   int64  `json:"user_id"` // Assuming UserID is a string type based on the pseudocode
}

func init() {
	callapi.RegisterHandler("get_login_info", GetLoginInfo)
}

func GetLoginInfo(client callapi.Client, Token string, BaseUrl string, message callapi.ActionMessage) (string, error) {

	var response LoginInfoResponse
	var botname string

	// Assuming 全局_botid is a global or environment variable
	globalBotID := config.BotID // Replace with the actual global variable or value
	globalBotID64, _ := strconv.ParseInt(globalBotID, 10, 64)
	botname = config.GetCustomBotName()

	response.Data = LoginInfoData{
		Nickname: botname,
		UserID:   globalBotID64,
	}
	response.Message = ""
	response.RetCode = 0
	response.Status = "ok"
	response.Echo = message.Echo

	// Convert the members slice to a map
	outputMap := structToMap(response)

	mylog.Printf("get_login_info: %+v\n", outputMap)

	err := client.SendMessage(outputMap)
	if err != nil {
		mylog.Printf("Error sending message via client: %v", err)
	} else {
		mylog.Printf("响应get_login_info: %+v", outputMap)
	}
	//把结果从struct转换为json
	result, err := json.Marshal(response)
	if err != nil {
		mylog.Printf("Error marshaling data: %v", err)
		//todo 符合onebotv11 ws返回的错误码
		return "", nil
	}
	return string(result), nil
}
