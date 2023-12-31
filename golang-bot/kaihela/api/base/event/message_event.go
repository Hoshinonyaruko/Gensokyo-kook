package event

type MessageTextEventFrame struct {
	Frame
	Data *MesssageTextEvent `json:"d"`
}
type MesssageTextEvent struct {
	BaseEvent
	Extra *MessageTextExtra `json:"extra"`
}
type MessageTextExtra struct {
	Type         int      `json:"type"`
	GuildId      string   `json:"guild_id"`
	ChannelName  string   `json:"channel_name"`
	Mention      string   `json:"mention"`
	MentionAll   bool     `json:"mention_all"`
	MentionRoles []string `json:"mention_roles"`
	MentionHere  bool     `json:"mention_here"`
	Author       User     `json:"author"`
}

type User struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	IdentifyNum    string `json:"identify_num"`
	Online         bool   `json:"online"`
	Avatar         string `json:"avatar"`
	VipAvatar      string `json:"vip_avatar"`
	Bot            bool   `json:"bot"`
	Status         int    `json:"status"`
	MobileVerified bool   `json:"mobile_verified"`
	Nickname       string `json:"nickname"`
	Roles          []int  `json:"roles"`
}

type MessageKMarkdownEvent struct {
	BaseEvent
	KMarkdownExtra `json:"extra"` // 修正标签中的引号
}

type TagInfo struct {
	Color string `json:"color"`
	Text  string `json:"text"`
}

type KMarkdown struct {
	RawContent      string `json:"raw_content"`
	MentionPart     []any  `json:"mention_part"`
	MentionRolePart []any  `json:"mention_role_part"`
}
type KMarkdownExtra struct {
	Type         int       `json:"type"`
	GuildID      string    `json:"guild_id"`
	ChannelName  string    `json:"channel_name"`
	Mention      []string  `json:"mention"`
	MentionAll   bool      `json:"mention_all"`
	MentionRoles []int     `json:"mention_roles"`
	MentionHere  bool      `json:"mention_here"`
	NavChannels  []string  `json:"nav_channels"`
	Code         string    `json:"code"`
	Author       User      `json:"author"`
	KMarkdown    KMarkdown `json:"kmarkdown"`
}
