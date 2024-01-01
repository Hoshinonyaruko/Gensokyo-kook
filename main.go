package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fatih/color"
	"github.com/gookit/event"
	"github.com/hoshinonyaruko/gensokyo-kook/Processor"
	"github.com/hoshinonyaruko/gensokyo-kook/config"
	"github.com/hoshinonyaruko/gensokyo-kook/handlers"
	"github.com/hoshinonyaruko/gensokyo-kook/httpapi"
	"github.com/hoshinonyaruko/gensokyo-kook/idmap"
	"github.com/hoshinonyaruko/gensokyo-kook/models"
	"github.com/hoshinonyaruko/gensokyo-kook/mylog"
	"github.com/hoshinonyaruko/gensokyo-kook/server"
	"github.com/hoshinonyaruko/gensokyo-kook/sys"
	"github.com/hoshinonyaruko/gensokyo-kook/template"
	"github.com/hoshinonyaruko/gensokyo-kook/url"
	"github.com/hoshinonyaruko/gensokyo-kook/webui"
	"github.com/hoshinonyaruko/gensokyo-kook/wsclient"
	"github.com/tencent-connect/botgo"

	"github.com/gin-gonic/gin"
	"github.com/idodo/golang-bot/kaihela/api/base"
	event2 "github.com/idodo/golang-bot/kaihela/api/base/event"
	"github.com/idodo/golang-bot/kaihela/api/helper"
	"github.com/idodo/golang-bot/kaihela/example/handler"
)

// 消息处理器，持有 openapi 对象
var p *Processor.Processors

func main() {
	// 定义faststart命令行标志。默认为false。
	fastStart := flag.Bool("faststart", false, "start without initialization if set")

	// 解析命令行参数到定义的标志。
	flag.Parse()

	// 检查是否使用了-faststart参数
	if !*fastStart {
		sys.InitBase() // 如果不是faststart模式，则执行初始化
	}
	if _, err := os.Stat("config.yml"); os.IsNotExist(err) {
		var ip string
		var err error
		// 检查操作系统是否为Android
		if runtime.GOOS == "android" {
			ip = "127.0.0.1"
		} else {
			// 获取内网IP地址
			ip, err = sys.GetLocalIP()
			if err != nil {
				log.Println("Error retrieving the local IP address:", err)
				ip = "127.0.0.1"
			}
		}
		// 将 <YOUR_SERVER_DIR> 替换成实际的内网IP地址 确保初始状态webui能够被访问
		configData := strings.Replace(template.ConfigTemplate, "<YOUR_SERVER_DIR>", ip, -1)

		// 将修改后的配置写入 config.yml
		err = os.WriteFile("config.yml", []byte(configData), 0644)
		if err != nil {
			log.Println("Error writing config.yml:", err)
			return
		}

		log.Println("请配置config.yml然后再次运行.")
		log.Print("按下 Enter 继续...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		os.Exit(0)
	}

	// 主逻辑
	// 加载配置
	conf, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	sys.SetTitle(conf.Settings.Title)
	webuiURL := config.ComposeWebUIURL(conf.Settings.Lotus)     // 调用函数获取URL
	webuiURLv2 := config.ComposeWebUIURLv2(conf.Settings.Lotus) // 调用函数获取URL

	var wsClients []*wsclient.WebSocketClient
	var nologin bool

	//logger
	logLevel := mylog.GetLogLevelFromConfig(config.GetLogLevel())
	loggerAdapter := mylog.NewMyLogAdapter(logLevel, config.GetSaveLogs())
	botgo.SetLogger(loggerAdapter)

	if conf.Settings.AppID == 12345 {
		// 输出天蓝色文本
		cyan := color.New(color.FgCyan)
		cyan.Printf("欢迎来到gensokyo-kook, 控制台地址: %s\n", webuiURL)
		log.Println("请完成机器人配置后重启框架。")

	} else {
		var response models.Response
		//创建api
		api := helper.NewApiHelper("/v3/user/me", conf.Settings.Token, conf.Settings.KaiheilaApi, "", "")

		configURL := config.GetDevelop_Acdir()
		if configURL == "" { // 执行API请求 显示机器人信息
			resp, err := api.Post()
			mylog.Printf("sent post:%s", api.String())
			if err != nil {
				return
			}
			mylog.Printf("Bot details:%s", string(resp))

			err = json.Unmarshal(resp, &response)
			if err != nil {
				// 处理解析错误
				log.Printf("解析机器人信息出错: %v", err)
				return
			}
		} else {
			log.Printf("自定义ac地址模式...请从日志手动获取bot的真实id并设置,不然at会不正常")
		}
		if !nologin {
			if configURL == "" { //初始化handlers
				handlers.BotID = response.Data.ID
			} else { //初始化handlers
				handlers.BotID = config.GetDevBotid()
			}
			botID64, err := strconv.ParseUint(handlers.BotID, 10, 64)

			// 获取 websocket 信息
			//这里类似intent订阅机制
			session := base.NewWebSocketSession(conf.Settings.Token, conf.Settings.KaiheilaApi, "./session.pid", "", 1)
			session.On(base.EventReceiveFrame, &handler.ReceiveFrameHandler{})
			session.On("GROUP*", &GroupEventHandler{})
			session.On("GROUP_9", &GroupTextEventHandler{Token: conf.Settings.Token, BaseUrl: conf.Settings.KaiheilaApi})
			session.On("PERSON_9", &PersonTextEventHandler{Token: conf.Settings.Token, BaseUrl: conf.Settings.KaiheilaApi})
			// 启动session.Start() 在一个新的goroutine
			go session.Start()

			if err != nil {
				mylog.Printf("Bot details err :%v", err)
				return
			}

			// 启动多个WebSocket客户端的逻辑
			if !allEmpty(conf.Settings.WsAddress) {
				wsClientChan := make(chan *wsclient.WebSocketClient, len(conf.Settings.WsAddress))
				errorChan := make(chan error, len(conf.Settings.WsAddress))
				// 定义计数器跟踪尝试建立的连接数
				attemptedConnections := 0
				for _, wsAddr := range conf.Settings.WsAddress {
					if wsAddr == "" {
						continue // Skip empty addresses
					}
					attemptedConnections++ // 增加尝试连接的计数
					go func(address string) {
						retry := config.GetLaunchReconectTimes()
						wsClient, err := wsclient.NewWebSocketClient(address, botID64, conf.Settings.Token, conf.Settings.KaiheilaApi, retry)
						if err != nil {
							log.Printf("Error creating WebSocketClient for address(连接到反向ws失败) %s: %v\n", address, err)
							errorChan <- err
							return
						}
						wsClientChan <- wsClient
					}(wsAddr)
				}
				// 获取连接成功后的wsClient
				for i := 0; i < attemptedConnections; i++ {
					select {
					case wsClient := <-wsClientChan:
						wsClients = append(wsClients, wsClient)
					case err := <-errorChan:
						log.Printf("Error encountered while initializing WebSocketClient: %v\n", err)
					}
				}

				// 确保所有尝试建立的连接都有对应的wsClient
				if len(wsClients) != attemptedConnections {
					log.Println("Error: Not all wsClients are initialized!(反向ws未设置或连接失败)")
					// 处理初始化失败的情况
					p = Processor.NewProcessorV2(conf.Settings.Token, conf.Settings.KaiheilaApi, &conf.Settings, botID64)
					//只启动正向
				} else {
					log.Println("All wsClients are successfully initialized.")
					// 所有客户端都成功初始化
					p = Processor.NewProcessor(conf.Settings.Token, conf.Settings.KaiheilaApi, &conf.Settings, wsClients, botID64)
				}
			} else if conf.Settings.EnableWsServer {
				log.Println("只启动正向ws")
				p = Processor.NewProcessorV2(conf.Settings.Token, conf.Settings.KaiheilaApi, &conf.Settings, botID64)
			}
		} else {
			// 设置颜色为红色
			red := color.New(color.FgRed)
			// 输出红色文本
			red.Println("请设置正确的appid、token、clientsecret再试")
		}

	}

	//创建idmap服务器 数据库
	idmap.InitializeDB()
	//创建webui数据库
	webui.InitializeDB()
	defer idmap.CloseDB()
	defer webui.CloseDB()

	// 根据 lotus 的值选择端口
	var serverPort string
	if !conf.Settings.Lotus {
		serverPort = conf.Settings.Port
	} else {
		serverPort = conf.Settings.BackupPort
	}
	var r *gin.Engine
	var hr *gin.Engine
	if config.GetDeveloperLog() { // 是否启动调试状态
		r = gin.Default()
		hr = gin.Default()
	} else {
		r = gin.New()
		r.Use(gin.Recovery()) // 添加恢复中间件，但不添加日志中间件
		hr = gin.New()
		hr.Use(gin.Recovery())
	}
	r.GET("/getid", server.GetIDHandler)
	r.Static("/channel_temp", "./channel_temp")

	//webui和它的api
	webuiGroup := r.Group("/webui")
	{
		webuiGroup.GET("/*filepath", webui.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
		webuiGroup.POST("/*filepath", webui.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
		webuiGroup.PUT("/*filepath", webui.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
		webuiGroup.DELETE("/*filepath", webui.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
		webuiGroup.PATCH("/*filepath", webui.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
	}

	//正向http api
	http_api_address := config.GetHttpAddress()
	if http_api_address != "" {
		mylog.Println("正向http api启动成功,监听" + http_api_address + "若有需要,请对外放通端口...")
		HttpApiGroup := hr.Group("/")
		{
			HttpApiGroup.GET("/*filepath", httpapi.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
			HttpApiGroup.POST("/*filepath", httpapi.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
			HttpApiGroup.PUT("/*filepath", httpapi.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
			HttpApiGroup.DELETE("/*filepath", httpapi.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
			HttpApiGroup.PATCH("/*filepath", httpapi.CombinedMiddleware(conf.Settings.Token, conf.Settings.KaiheilaApi))
		}
	}
	//正向ws
	if conf.Settings.AppID != 12345 {
		if conf.Settings.EnableWsServer {
			wspath := config.GetWsServerPath()
			if wspath == "nil" {
				r.GET("", server.WsHandlerWithDependencies(conf.Settings.Token, conf.Settings.KaiheilaApi, p))
				mylog.Println("正向ws启动成功,监听0.0.0.0:" + serverPort + "请注意设置ws_server_token(可空),并对外放通端口...")
			} else {
				r.GET("/"+wspath, server.WsHandlerWithDependencies(conf.Settings.Token, conf.Settings.KaiheilaApi, p))
				mylog.Println("正向ws启动成功,监听0.0.0.0:" + serverPort + "/" + wspath + "请注意设置ws_server_token(可空),并对外放通端口...")
			}
		}
	}
	r.POST("/url", url.CreateShortURLHandler)
	r.GET("/url/:shortURL", url.RedirectFromShortURLHandler)
	if config.GetIdentifyFile() {
		appIDStr := config.GetAppIDStr()
		fileName := appIDStr + ".json"
		r.GET("/"+fileName, func(c *gin.Context) {
			content := fmt.Sprintf(`{"bot_appid":%d}`, config.GetAppID())
			c.Header("Content-Type", "application/json")
			c.String(200, content)
		})
	}
	// 创建一个http.Server实例（主服务器）
	httpServer := &http.Server{
		Addr:    "0.0.0.0:" + serverPort,
		Handler: r,
	}
	mylog.Printf("gin运行在%v端口", serverPort)
	// 在一个新的goroutine中启动主服务器
	go func() {
		if serverPort == "443" {
			// 使用HTTPS
			crtPath := config.GetCrtPath()
			keyPath := config.GetKeyPath()
			if crtPath == "" || keyPath == "" {
				log.Fatalf("crt or key path is missing for HTTPS")
				return
			}
			if err := httpServer.ListenAndServeTLS(crtPath, keyPath); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen (HTTPS): %s\n", err)
			}
		} else {
			// 使用HTTP
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		}
	}()

	// 如果主服务器使用443端口，同时在一个新的goroutine中启动444端口的HTTP服务器 todo 更优解
	if serverPort == "443" {
		go func() {
			// 创建另一个http.Server实例（用于444端口）
			httpServer444 := &http.Server{
				Addr:    "0.0.0.0:444",
				Handler: r,
			}

			// 启动444端口的HTTP服务器
			if err := httpServer444.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen (HTTP 444): %s\n", err)
			}
		}()
	}
	// 创建 httpapi 的http server
	if http_api_address != "" {
		go func() {
			// 创建一个http.Server实例（Http Api服务器）
			httpServerHttpApi := &http.Server{
				Addr:    http_api_address,
				Handler: hr,
			}
			// 使用HTTP
			if err := httpServerHttpApi.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("http apilisten: %s\n", err)
			}
		}()
	}

	// 使用color库输出天蓝色的文本
	cyan := color.New(color.FgCyan)
	cyan.Printf("欢迎来到gensokyo-kook, 控制台地址: %s\n", webuiURL)
	cyan.Printf("%s\n", template.Logo)
	cyan.Printf("欢迎来到gensokyo-kook, 公网控制台地址(需开放端口): %s\n", webuiURLv2)

	// 使用通道来等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞主线程，直到接收到信号
	<-sigCh

	// 关闭 WebSocket 连接
	// wsClients 是一个 *wsclient.WebSocketClient 的切片
	for _, client := range wsClients {
		err := client.Close()
		if err != nil {
			log.Printf("Error closing WebSocket connection: %v\n", err)
		}
	}

	// 关闭BoltDB数据库
	url.CloseDB()
	idmap.CloseDB()

	// 在关闭WebSocket客户端之前
	for _, wsClient := range p.WsServerClients {
		if err := wsClient.Close(); err != nil {
			log.Printf("Error closing WebSocket server client: %v\n", err)
		}
	}

	// 使用一个5秒的超时优雅地关闭Gin服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
}

type GroupTextEventHandler struct {
	Token   string
	BaseUrl string
}

func (gteh *GroupTextEventHandler) Handle(e event.Event) error {
	mylog.Printf("bot[%v]event: %+v, 收到频道内的文字消息.", handlers.BotID, e.Data())

	err := func() error {
		if _, ok := e.Data()[base.EventDataFrameKey]; !ok {
			mylog.Errorf("data has no frame field")
			return nil
		}
		frame := e.Data()[base.EventDataFrameKey].(*event2.FrameMap)
		data, err := sonic.Marshal(frame.Data)
		if err != nil {
			return err
		}
		msgEvent := &event2.MessageKMarkdownEvent{}
		err = sonic.Unmarshal(data, msgEvent)
		mylog.Printf("Received json event:%+v", msgEvent)
		if err != nil {
			return err
		}
		if config.GetIgnoreBotMessage() {
			if msgEvent.Author.Bot {
				mylog.Errorf("message form bot")
				return nil
			}
		}
		//投递给信息处理器
		return p.ProcessGuildNormalMessage(msgEvent)
	}()
	if err != nil {
		mylog.Errorf("GroupTextEventHandler err")
	}

	return nil
}

type GroupEventHandler struct {
}

func (ge *GroupEventHandler) Handle(e event.Event) error {
	mylog.Printf("event: %+v, 收到频道内的事件消息.", e.Data())
	return nil
}

type PersonTextEventHandler struct {
	Token   string
	BaseUrl string
}

func (gteh *PersonTextEventHandler) Handle(e event.Event) error {
	mylog.Printf("bot[%v]event: %+v, 收到私信内的文字消息.", handlers.BotID, e.Data())
	err := func() error {
		if _, ok := e.Data()[base.EventDataFrameKey]; !ok {
			mylog.Errorf("data has no frame field")
			return nil
		}
		frame := e.Data()[base.EventDataFrameKey].(*event2.FrameMap)
		data, err := sonic.Marshal(frame.Data)
		if err != nil {
			return err
		}
		msgEvent := &event2.MessageKMarkdownEvent{}
		err = sonic.Unmarshal(data, msgEvent)
		mylog.Printf("Received json event:%+v", msgEvent)
		if err != nil {
			return err
		}
		if config.GetIgnoreBotMessage() {
			if msgEvent.Author.Bot {
				mylog.Errorf("message form bot")
				return nil
			}
		}
		//投递给信息处理器
		return p.ProcessChannelDirectMessage(msgEvent)
	}()
	if err != nil {
		mylog.Errorf("GroupTextEventHandler err")
	}

	return nil
}

// allEmpty checks if all the strings in the slice are empty.
func allEmpty(addresses []string) bool {
	for _, addr := range addresses {
		if addr != "" {
			return false
		}
	}
	return true
}
