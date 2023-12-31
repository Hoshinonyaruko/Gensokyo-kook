package event

type HelloSignal struct {
	Frame
	Data *HelloData `json:"d"`
}

type HelloData struct {
	Code      string `json:"code"`
	SessionId string `json:"session_id"`
}

type PingSignal struct {
	Frame
	SerialNumber int64 `json:"sn"`
}

type PongSignal struct {
	Frame
}

type ResumeSignal struct {
	Frame
	SerialNumber int64 `json:"sn"`
}

type ReconnectSignal struct {
	Frame
	Data *ReconnectData `json:"d"`
}

type ReconnectData struct {
	Code int    `json:"code"`
	Err  string `json:"err"`
}

type ResumeACKSignal struct {
	Frame
	Data *ResumeACKData `json:"d"`
}

type ResumeACKData struct {
	SessionId string `json:"session_id"`
}
