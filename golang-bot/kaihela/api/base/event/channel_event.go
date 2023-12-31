package event

type ChannelAddUserEventFrame struct {
	Frame
	ChannelAddUserEvent
}

type ChannelAddUserEvent struct {
	BaseEvent
	Extra *ChannelAddUserExtra `json:"extra"` // 修改此处，将extra首字母大写
}

type Emoji struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type ChannelAddUserExtra struct {
	Type string `json:"type"`
	Body struct {
		ChannelId string `json:"channel_id"`
		Emoji     *Emoji `json:"emoji"` // 要在JSON中包含这个字段，需要添加标签
		UserId    string `json:"user_id"`
		MsgId     string `json:"msg_id"`
	} `json:"body"` // 修改此处，正确的标签语法
}
