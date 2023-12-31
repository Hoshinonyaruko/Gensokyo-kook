package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/handlers"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/idodo/golang-bot/kaihela/api/helper"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
	"github.com/tencent-connect/botgo/dto"
)

//go:embed dist/*
//go:embed dist/css/*
//go:embed dist/fonts/*
//go:embed dist/icons/*
//go:embed dist/js/*
var content embed.FS

// NewCombinedMiddleware 创建并返回一个带有依赖的中间件闭包
func CombinedMiddleware(Token string, BaseUrl string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/webui/api") {
			// 处理API请求
			appIDStr := handlers.BotID
			//todo 完善logs的 get方法 来获取历史日志
			// 检查路径是否匹配 `/api/{uin}/process/logs`
			if strings.HasPrefix(c.Param("filepath"), "/api/") && strings.HasSuffix(c.Param("filepath"), "/process/logs") {
				if c.GetHeader("Upgrade") == "websocket" {
					mylog.WsHandlerWithDependencies(c)
				} else {
					getProcessLogs(c)
				}
				return
			}
			//主页日志
			if c.Param("filepath") == "/api/logs" {
				if c.GetHeader("Upgrade") == "websocket" {
					mylog.WsHandlerWithDependencies(c)
				} else {
					getProcessLogs(c)
				}
				return
			}
			// 如果请求路径与appIDStr匹配，并且请求方法为PUT
			if c.Param("filepath") == appIDStr && c.Request.Method == http.MethodPut {
				HandleAppIDRequest(c)
				return
			}
			//获取状态
			if c.Param("filepath") == "/api/"+appIDStr+"/process/status" {
				HandleProcessStatusRequest(c)
				return
			}
			//获取机器人列表
			if c.Param("filepath") == "/api/accounts" {
				HandleAccountsRequest(c)
				return
			}
			//获取当前选中机器人的配置
			if c.Param("filepath") == "/api/"+appIDStr+"/config" && c.Request.Method == http.MethodGet {
				AccountConfigReadHandler(c)
				return
			}
			//删除当前选中机器人的配置并生成新的配置
			if c.Param("filepath") == "/api/"+appIDStr+"/config" && c.Request.Method == http.MethodDelete {
				handleDeleteConfig(c)
				return
			}
			//结束当前实例的进程
			if c.Param("filepath") == "/api/"+appIDStr+"/process" && c.Request.Method == http.MethodDelete {
				// 正常退出
				os.Exit(0)
				return
			}
			//进程监控
			if c.Param("filepath") == "/api/status" && c.Request.Method == http.MethodGet {
				// 检查操作系统是否不为Android
				if runtime.GOOS != "android" {
					handleSysInfo(c)
				}
				return
			}
			//更新当前选中机器人的配置并重启应用(保持地址不变)
			if c.Param("filepath") == "/api/"+appIDStr+"/config" && c.Request.Method == http.MethodPatch {
				handlePatchConfig(c)
				return
			}
			// 处理/api/login的POST请求
			if c.Param("filepath") == "/api/login" && c.Request.Method == http.MethodPost {
				HandleLoginRequest(c)
				return
			}
			// 处理/api/check-login-status的GET请求
			if c.Param("filepath") == "/api/check-login-status" && c.Request.Method == http.MethodGet {
				HandleCheckLoginStatusRequest(c)
				return
			}
			// 根据api名称处理请求
			if c.Param("filepath") == "/api/"+appIDStr+"/api" && c.Request.Method == http.MethodPost {
				apiName := c.Query("name")
				switch apiName {
				case "get_guild_list":
					// 处理获取群组列表的请求
					handleGetGuildList(c, Token, BaseUrl)
				case "get_channel_list":
					// 处理获取频道列表的请求
					handleGetChannelList(c, Token, BaseUrl)
				case "send_guild_channel_message":
					// 调用处理发送消息的函数
					handleSendGuildChannelMessage(c, Token, BaseUrl)
				default:
					// 处理其他或未知的api名称
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API name"})
				}
			}
			//根据api名称处理请求 get
			if c.Param("filepath") == "/api/"+appIDStr+"/api" && c.Request.Method == http.MethodGet {
				apiName := c.Query("name")
				switch apiName {
				case "get_icon":
					//获取开黑啦图标返回给前端
					handleGetIcon(c)
				}
				return
			}
			// 如果还有其他API端点，可以在这里继续添加...
		} else {
			// 否则，处理静态文件请求
			// 如果请求是 "/webui/" ，默认为 "index.html"
			filepathRequested := c.Param("filepath")
			if filepathRequested == "" || filepathRequested == "/" {
				filepathRequested = "index.html"
			}

			// 使用 embed.FS 读取文件内容
			filepathRequested = strings.TrimPrefix(filepathRequested, "/")
			data, err := content.ReadFile("dist/" + filepathRequested)
			if err != nil {
				fmt.Println("Error reading file:", err)
				c.Status(http.StatusNotFound)
				return
			}

			mimeType := getContentType(filepathRequested)

			c.Data(http.StatusOK, mimeType, data)
		}
		// 调用c.Next()以继续处理请求链
		c.Next()
	}
}

// SendMessageRequest 定义了发送消息请求的数据结构
type SendMessageRequest struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

// handleSendGuildChannelMessage 处理发送消息到公会频道的请求
func handleSendGuildChannelMessage(c *gin.Context, Token string, BaseUrl string) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 使用新的API方式发送消息
	api := helper.NewApiHelper("/v3/message/create", Token, BaseUrl, "", "")

	// 构造请求数据映射
	data := map[string]string{
		"type":       "1",         // 假设为文本消息类型
		"channel_id": req.ID,      // 使用请求中的channelID
		"content":    req.Message, // 使用请求中的消息内容
	}

	// 序列化请求数据为JSON
	requestDataByte, err := sonic.Marshal(data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize request"})
		return
	}

	// 发送POST请求
	resp, err := api.SetBody(requestDataByte).Post()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to send message",
			"details": err.Error(),
		})
		return
	}

	// 如果消息发送成功，返回一个成功的响应
	c.JSON(http.StatusOK, gin.H{
		"message": "Message sent successfully",
		"data":    string(resp),
	})
}

type RequestParams struct {
	Page     string `json:"page"`
	PageSize string `json:"page_size"`
	Sort     string `json:"sort"`
}

// handleGetGuildList 处理获取服务器列表的请求
func handleGetGuildList(c *gin.Context, Token string, BaseUrl string) {
	var params RequestParams

	// 解析请求体中的 JSON 数据
	if err := c.BindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 构建请求到后端API的参数
	params2 := map[string]string{
		"page":      params.Page,
		"page_size": params.PageSize,
		"sort":      params.Sort,
	}
	api := helper.NewApiHelper("/v3/guild/list", Token, BaseUrl, "", "")

	// 调用新的后端API获取数据
	guilds, err := FetchGuildsWebUI(api, params2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 转换数据以适应前端结构
	guildList := make([]map[string]interface{}, len(guilds))
	for i, guild := range guilds {
		// 去除 URL 中的 "?x-oss-process=style/icon" 部分
		iconURL := strings.Split(guild.Icon, "?")[0]
		// 对图标 URL 进行 URL 编码
		encodedIconURL := url.QueryEscape(iconURL)
		port := config.GetPortValue()
		// 构造完整的 API URL
		apiURL := fmt.Sprintf("http://127.0.0.1:%v/webui/api/%v/api?name=get_icon&url=%s", port, handlers.BotID, encodedIconURL)
		guildList[i] = map[string]interface{}{
			"id":             guild.ID,
			"name":           guild.Name,
			"icon":           apiURL,
			"owner_id":       "",
			"owner":          false,
			"member_count":   123,
			"max_members":    456,
			"description":    guild.Topic,
			"joined_at":      "123",
			"channels":       "",
			"union_world_id": "",
			"union_org_id":   "",
		}
	}

	// 假设可以从 somewhere 获取 totalPages
	totalPages := 1000

	// 返回数据给前端，匹配前端期望的结构
	c.JSON(http.StatusOK, gin.H{
		"data":       guildList,
		"totalPages": totalPages, // 需要后端提供或计算出总页数
	})
}

// handleGetChannelList 处理获取服务器子频道列表的请求
func handleGetChannelList(c *gin.Context, Token string, BaseUrl string) {
	// 提取前端发来的 pager 数据，其中after参数作为channelID使用
	var pager dto.GuildPager
	if err := c.ShouldBindJSON(&pager); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果after是空字符串，则设置为默认值（如"0"，或者可适当调整）
	if pager.After == "" {
		pager.After = "0"
	}
	apiHelper := helper.NewApiHelper("/v3/channel/list", Token, BaseUrl, "", "")
	// 初始化 API 帮助类

	// 调用 FetchChannels 函数获取频道列表
	channels, err := handlers.FetchChannels(apiHelper, pager.After) // 这里的pager.After实际上作为guildID
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 将获取到的频道数据转换为前端需要的格式
	channelList := make([]map[string]interface{}, len(channels))
	for i, channel := range channels {
		channelList[i] = map[string]interface{}{
			"id":               channel.ID,
			"name":             channel.Name,
			"type":             channel.Type,
			"position":         "",         // 或者使用默认值
			"parent_id":        "",         // 或者使用默认值
			"owner_id":         "",         // 或者使用默认值
			"sub_type":         "",         // 或者使用默认值
			"private_type":     "",         // 或者使用默认值
			"private_user_ids": []string{}, // 空数组
			"speak_permission": "",         // 或者使用默认值
			"application_id":   "",         // 或者使用默认值
			"permissions":      "",         // 或者使用默认值
			"op_user_id":       "",         // 或者使用默认值
			// ... 其他需要的字段
		}
	}

	// 假设可以从 somewhere 获取 totalPages
	totalPages := 100 // 或者根据实际情况计算

	// 返回数据给前端，匹配前端期望的结构
	c.JSON(http.StatusOK, gin.H{
		"data":       channelList,
		"totalPages": totalPages, // 总页数可以是后端提供或计算出的
	})
}

func getContentType(path string) string {
	// todo 根据需要增加更多的 MIME 类型
	switch filepath.Ext(path) {
	case ".html":
		return "text/html"
	case ".js":
		return "application/javascript"
	case ".css":
		return "text/css"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "text/plain"
	}
}

type ResponseData struct {
	UIN      int64  `json:"uin"`
	Password string `json:"password"`
	Protocol int    `json:"protocol"`
}

type RequestData struct {
	Password string `json:"password"`
}

func HandleAccountsRequest(c *gin.Context) {
	responseData := []gin.H{
		{
			"uin":             handlers.BotID,
			"predefined":      false,
			"process_created": true,
		},
	}

	c.JSON(http.StatusOK, responseData)
}

func HandleProcessStatusRequest(c *gin.Context) {
	responseData := gin.H{
		"status":     "running",
		"total_logs": 0,
		"restarts":   0,
		"qr_uri":     nil,
		"details": gin.H{
			"pid":         0,
			"status":      "running",
			"memory_used": 19361792,          // 示例内存使用量
			"cpu_percent": 0.0,               // 示例CPU使用率
			"start_time":  time.Now().Unix(), // 10位时间戳
		},
	}
	c.JSON(http.StatusOK, responseData)
}

// 待完善 从mylog通道取出日志信息,然后一股脑返回
func getProcessLogs(c *gin.Context) {
	c.JSON(200, []interface{}{})
}

func HandleAppIDRequest(c *gin.Context) {
	appIDStr := handlers.BotID

	// 将 appIDStr 转换为 int64
	uin, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 解析请求体中的JSON数据
	var requestData RequestData
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// 创建响应数据
	responseData := ResponseData{
		UIN:      uin,
		Password: requestData.Password,
		Protocol: 5,
	}

	// 发送响应
	c.JSON(http.StatusOK, responseData)
}

// AccountConfigReadHandler 是用来处理读取配置文件的HTTP请求的
func AccountConfigReadHandler(c *gin.Context) {
	// 读取config.yml文件
	yamlFile, err := os.ReadFile("config.yml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to read config file"})
		return
	}

	// 创建JSON响应
	jsonResponse := gin.H{
		"content": string(yamlFile),
	}

	// 将JSON响应发送回客户端
	c.JSON(http.StatusOK, jsonResponse)
}

// 删除配置的处理函数
func handleDeleteConfig(c *gin.Context) {
	// 这里调用删除配置的函数
	err := config.DeleteConfig() // 假设DeleteConfig接受文件路径作为参数
	if err != nil {
		// 如果删除出现错误，返回服务器错误状态码
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 删除成功，返回204 No Content状态码
	c.Status(http.StatusNoContent)
}

// handlePatchConfig 用来处理PATCH请求，更新config.yml文件的内容
func handlePatchConfig(c *gin.Context) {
	// 解析请求体中的JSON数据
	var jsonBody struct {
		Content string `json:"content"`
	}
	if err := c.BindJSON(&jsonBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 使用WriteYAMLToFile将content写入config.yml
	if err := config.WriteYAMLToFile(jsonBody.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write to config file"})
		return
	}

	// 如果没有错误，返回成功响应
	c.JSON(http.StatusOK, gin.H{"message": "Config updated successfully"})
}

// HandleLoginRequest处理登录请求
func HandleLoginRequest(c *gin.Context) {
	var json struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if checkCredentials(json.Username, json.Password) {
		// 如果验证成功，设置cookie
		cookieValue, err := GenerateCookie()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate cookie"})
			return
		}

		c.SetCookie("login_cookie", cookieValue, 3600*24, "/", "", false, true)

		c.JSON(http.StatusOK, gin.H{
			"isLoggedIn": true,
			"cookie":     cookieValue,
		})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"isLoggedIn": false,
		})
	}
}

func checkCredentials(username, password string) bool {
	serverUsername := config.GetServerUserName()
	serverPassword := config.GetServerUserPassword()

	return username == serverUsername && password == serverPassword
}

// HandleCheckLoginStatusRequest 检查登录状态的处理函数
func HandleCheckLoginStatusRequest(c *gin.Context) {
	// 从请求中获取cookie
	cookieValue, err := c.Cookie("login_cookie")
	if err != nil {
		// 如果cookie不存在，而不是返回BadRequest(400)，我们返回一个OK(200)的响应
		c.JSON(http.StatusOK, gin.H{"isLoggedIn": false, "error": "Cookie not provided"})
		return
	}

	// 使用ValidateCookie函数验证cookie
	isValid, err := ValidateCookie(cookieValue)
	if err != nil {
		switch err {
		case ErrCookieNotFound:
			c.JSON(http.StatusOK, gin.H{"isLoggedIn": false, "error": "Cookie not found"})
		case ErrCookieExpired:
			c.JSON(http.StatusOK, gin.H{"isLoggedIn": false, "error": "Cookie has expired"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"isLoggedIn": false, "error": "Internal server error"})
		}
		return
	}

	if isValid {
		c.JSON(http.StatusOK, gin.H{"isLoggedIn": true})
	} else {
		c.JSON(http.StatusOK, gin.H{"isLoggedIn": false, "error": "Invalid cookie"})
	}
}

func handleSysInfo(c *gin.Context) {
	// 获取CPU使用率
	cpuPercent, _ := cpu.Percent(time.Second, false)

	// 获取内存信息
	vmStat, _ := mem.VirtualMemory()

	// 获取磁盘使用情况
	diskStat, _ := disk.Usage("/")

	// 获取系统启动时间
	bootTime, _ := host.BootTime()

	// 获取当前进程信息
	proc, _ := process.NewProcess(int32(os.Getpid()))
	procPercent, _ := proc.CPUPercent()
	memInfo, _ := proc.MemoryInfo()
	procStartTime, _ := proc.CreateTime()

	// 构造返回的JSON数据
	sysInfo := gin.H{
		"cpu_percent": cpuPercent[0], // CPU使用率
		"memory": gin.H{
			"total":     vmStat.Total,       // 总内存
			"available": vmStat.Available,   // 可用内存
			"percent":   vmStat.UsedPercent, // 内存使用率
		},
		"disk": gin.H{
			"total":   diskStat.Total,       // 磁盘总容量
			"free":    diskStat.Free,        // 磁盘剩余空间
			"percent": diskStat.UsedPercent, // 磁盘使用率
		},
		"boot_time": bootTime, // 系统启动时间
		"process": gin.H{
			"pid":         proc.Pid,      // 当前进程ID
			"status":      "running",     // 进程状态，这里假设为运行中
			"memory_used": memInfo.RSS,   // 进程使用的内存
			"cpu_percent": procPercent,   // 进程CPU使用率
			"start_time":  procStartTime, // 进程启动时间
		},
	}
	// 返回JSON数据
	c.JSON(http.StatusOK, sysInfo)
}

// FetchGuildsWebUI 用于获取当前用户加入的服务器列表，接受自定义参数
func FetchGuildsWebUI(api *helper.ApiHelper, params map[string]string) ([]handlers.GuildData, error) {
	api.SetQuery(params) // 设置查询参数
	mylog.Printf("设置的获取频道列表参数:%v", params)
	// 发起请求获取服务器列表
	resp, err := api.Get()
	if err != nil {
		mylog.Printf("获取服务器列表出错%v", resp)
		return nil, err
	}

	// 解析响应数据
	var guildListResponse struct {
		Code    int
		Message string
		Data    struct {
			Items []handlers.GuildData
		}
	}
	err = json.Unmarshal(resp, &guildListResponse)
	if err != nil {
		return nil, err
	}

	return guildListResponse.Data.Items, nil
}

// handleGetIcon 代理获取图像的请求
func handleGetIcon(c *gin.Context) {
	// 从查询参数中获取图像 URL
	iconURL := c.Query("url")
	if iconURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL parameter is required"})
		return
	}

	// 发起 HTTP 请求获取图像
	resp, err := http.Get(iconURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching image"})
		return
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching image"})
		return
	}

	// 读取响应体内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading image data"})
		return
	}

	// 打印 data 的长度
	fmt.Printf("Fetched image data length: %d bytes\n", len(data))

	// 将原始响应的内容类型设置到当前响应中
	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))

	// 将图像数据作为响应返回
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write(data)
}
