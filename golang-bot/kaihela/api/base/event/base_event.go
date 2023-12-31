package event

const (
	SIG_EVENT      int32 = 0
	SIG_HELLO      int32 = 1
	SIG_PING       int32 = 2
	SIG_PONG       int32 = 3
	SIG_RESUME     int32 = 4
	SIG_RECONNECT  int32 = 5
	SIG_RESUME_ACK int32 = 6
)

type BaseEvent struct {
	ChannelType  string `json:"channel_type"`
	Type         int    `json:"type"`
	TargetId     string `json:"target_id"`
	AuthorId     string `json:"author_id"`
	Content      string `json:"content"`
	MsgId        string `json:"msg_id"`
	MsgTimestamp int64  `json:"msg_timestamp"`
	Nonce        string `json:"nonce"`
	SerialNumber int64  `json:"sn"`
}
