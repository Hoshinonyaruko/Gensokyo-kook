package template

const ConfigTemplate = `
version: 1
settings:
  #反向ws设置
  ws_address : ["ws://<YOUR_WS_ADDRESS>:<YOUR_WS_PORT>"] # WebSocket服务的地址 支持多个["","",""]
  ws_token : ["","",""]              #连接wss地址时服务器所需的token,按顺序一一对应,如果是ws地址,没有密钥,请留空.
  token : "<YOUR_APP_TOKEN>"                          # 你的应用令牌
  kaiheila_api : "https://www.kookapp.cn/api"         #api地址
  restart_time : 86400                               #每天自己重启自己一下,kook总是断线.仅支持win,linux请修改为0.

  global_channel_to_group: true                      # 是否将频道转换成群 默认true
  global_private_to_channel: false                   # 是否将私聊转换成频道 如果是群场景 会将私聊转为群(方便提审\测试)
  array: false                                       # 连接trss云崽请开启array
  hash_id : true                                     # 使用hash来进行idmaps转换,可以让user_id不是123开始的递增值

  server_dir: "<YOUR_SERVER_DIR>"                    # 提供图片上传服务的服务器(图床)需要带端口号. 如果需要发base64图,需为公网ip,且开放对应端口
  port: "15630"                                      # idmaps和图床对外开放的端口号
  backup_port : "5200"                               # 当totus为ture时,port值不再是本地webui的端口,使用lotus_Port来访问webui

  lotus: false                                       # lotus特性默认为false,当为true时,将会连接到另一个lotus为false的gensokyo-kook。
                                                     # 使用它提供的图床和idmaps服务(场景:同一个机器人在不同服务器运行,或内网需要发送base64图)。
                                                     # 如果需要发送base64图片,需要设置正确的公网server_dir和开放对应的port
  lotus_password : ""                                # lotus鉴权 设置后,从gsk需要保持相同密码来访问主gsk

  #增强配置项                                           

  image_sizelimit : 0               #代表kb 腾讯api要求图片1500ms完成传输 如果图片发不出 请提升上行或设置此值 默认为0 不压缩
  image_limit : 100                 #每分钟上传的最大图片数量,可自行增加
  master_id : ["1","2"]             #群场景尚未开放获取管理员和列表能力,手动从日志中获取需要设置为管理,的user_id并填入(适用插件有权限判断场景)
  record_sampleRate : 24000         #语音文件的采样率 最高48000 默认24000 单位Khz
  record_bitRate : 24000            #语音文件的比特率 默认25000 代表 25 kbps 最高无限 请根据带宽 您发送的实际码率调整
  ignore_bot_message : true         #忽略来自机器人自身和其他机器人的信息(如果你希望机器人回复其他机器人信息,请处理好机器人收到自身信息的逻辑,谨防死循环!)
  reconnect_times : 100             #反向ws连接失败后的重试次数,希望一直重试,可设置9999
  heart_beat_interval : 10          #反向ws心跳间隔 单位秒 推荐5-10
  launch_reconnect_times : 1        #启动时尝试反向ws连接次数,建议先打开应用端再开启gensokyo-kook,因为启动时连接会阻塞webui启动,默认只连接一次,可自行增大
  native_ob11 : true               #如果你的机器人收到事件报错,请开启此选项增加兼容性
  ob11_int32 : true                     #部分不支持group_id是int64的应用端,将int64转换为更短的int32
  self_introduce : ["",""]          #自我介绍,可设置多个随机发送,当不为空时,机器人被邀入群会发送自定义自我介绍 需手动添加新textintent   - "GroupAddRobotEventHandler"   - "GroupDelRobotEventHandler"

  #正向ws设置
  ws_server_path : "ws"             #默认监听0.0.0.0:port/ws_server_path 若有安全需求,可不放通port到公网,或设置ws_server_token 若想监听/ 可改为"",若想监听到不带/地址请写nil
  enable_ws_server: true            #是否启用正向ws服务器 监听server_dir:port/ws_server_path
  ws_server_token : "12345"         #正向ws的token 不启动正向ws可忽略 可为空

  #日志类

  developer_log : false             #开启开发者日志 默认关闭
  log_level : 1                     # 0=debug 1=info 2=warning 3=error 默认1
  save_logs : false                 #自动储存日志

  #webui设置

  server_user_name : "useradmin"    #默认网页面板用户名
  server_user_password : "admin"    #默认网页面板密码

  #指令过滤类

  remove_prefix : false             #是否忽略公域机器人指令前第一个/
  remove_at : false                 #是否忽略公域机器人指令前第一个[CQ:aq,qq=机器人] 场景(公域机器人,但插件未适配at开头)
  remove_bot_at_group : true        #因为群聊机器人不支持发at,开启本开关会自动隐藏群机器人发出的at(不影响频道场景)
  add_at_group : false              #自动在群聊指令前加上at,某些机器人写法特别,必须有at才反应时,请打开,默认请关闭(如果需要at,不需要at指令混杂,请优化代码适配群场景,群场景目前没有at概念)

  white_prefix_mode : false         #公域 过审用 指令白名单模式开关 如果审核严格 请开启并设置白名单指令 以白名单开头的指令会被通过,反之被拦截
  white_prefixs : [""]              #可设置多个 比如设置 机器人 测试 则只有信息以机器人 测试开头会相应 remove_prefix remove_at 需为true时生效
  white_bypass : []                 #格式[1,2,3],白名单不生效的群或用户(私聊时),用于设置自己的灰度沙箱群/灰度沙箱私聊,避免开发测试时反复开关白名单的不便,请勿用于生产环境.
  white_enable : [true,true,true,true,true] #指令白名单生效范围,5个分别对应,频道公(ATMessageEventHandler),频道私(CreateMessageHandler),频道私聊,群,群私聊,改成false,这个范围就不生效指令白名单(使用场景:群全量,频道私域的机器人,或有私信资质的机器人)
  white_bypass_reverse : false      #反转white_bypass的效果,可仅在white_bypass应用white_prefix_mode,场景:您的不同用户群,可以开放不同层次功能,便于您的运营和规化(测试/正式环境)
  No_White_Response : ""            #默认不兜底,强烈建议设置一个友善的兜底回复,告知审核机器人已无隐藏指令,如:你输入的指令不对哦,@机器人来获取可用指令

  black_prefix_mode : false         #公私域 过审用 指令黑名单模式开关 过滤被审核打回的指令不响应 无需改机器人后端
  black_prefixs : [""]              #可设置多个 比如设置 查询 则查询开头的信息均被拦截 防止审核失败
  alias : ["",""]                   #两两成对,指令替换,"a","b","c","d"代表将a开头替换为b开头,c开头替换为d开头.

  visual_prefixs :                  #虚拟前缀 与white_prefixs配合使用 处理流程自动忽略该前缀 remove_prefix remove_at 需为true时生效
  - prefix: ""                      #虚拟前缀开头 例 你有3个指令 帮助 测试 查询 将 prefix 设置为 工具类 后 则可通过 工具类 帮助 触发机器人
    whiteList: [""]                 #开关状态取决于 white_prefix_mode 为每一个二级指令头设计独立的白名单
    No_White_Response : ""                                   
  - prefix: ""
    whiteList: [""]
    No_White_Response : "" 
  - prefix: ""
    whiteList: [""]
    No_White_Response : "" 

  #开发增强类
  send_delay : 300                  #单位 毫秒 默认300ms 可以视情况减少到100或者50

  title : "gensokyo-kook © 2023 - Hoshinonyaruko"              #程序的标题 如果多个机器人 可根据标题区分
  custom_bot_name : "gensokyo-kook全域机器人"                   #自定义机器人名字,会在api调用中返回,默认gensokyo-kook全域机器人
 
  forward_msg_limit : 3             #发送折叠转发信息时的最大限制条数 若要发转发信息 请设置lazy_message_id为true
  
  #bind指令类 

  bind_prefix : "/bind"             #需设置   #增强配置项  master_id 可触发
  me_prefix : "/me"                 #需设置   #增强配置项  master_id 可触发

  #HTTP API配置

  #正向http
  http_address: ""                  #http监听地址 与websocket独立 示例:0.0.0.0:5700 为空代表不开启
  http_version : 11                 #暂时只支持11
  http_timeout: 5                   #反向 HTTP 超时时间, 单位秒，<5 时将被忽略

  #反向http
  post_url: [""]                    #反向HTTP POST地址列表 为空代表不开启 示例:http://192.168.0.100:5789
  post_secret: [""]                 #密钥
  post_max_retries: [3]             #最大重试,0 时禁用
  post_retries_interval: [1500]     #重试时间,单位毫秒,0 时立即
`
const Logo = `
'                                                                                                      
'    ,hakurei,                                                      ka                                  
'   ho"'     iki                                                    gu                                  
'  ra'                                                              ya                                  
'  is              ,kochiya,    ,sanae,    ,Remilia,   ,Scarlet,    fl   and  yu        ya   ,Flandre,   
'  an      Reimu  'Dai   sei  yas     aka  Rei    sen  Ten     shi  re  sca    yu      ku'  ta"     "ko  
'  Jun        ko  Kirisame""  ka       na    Izayoi,   sa       ig  Koishi       ko   mo'   ta       ga  
'   you.     rei  sui   riya  ko       hi  Ina    baI  'ran   you   ka  rlet      komei'    "ra,   ,sa"  
'     "Marisa"      Suwako    ji       na   "Sakuya"'   "Cirno"'    bu     sen     yu''        Satori  
'                                                                                ka'                   
'                                                                               ri'                    
`
