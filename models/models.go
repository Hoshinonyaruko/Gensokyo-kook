package models

type Response struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    UserData `json:"data"`
}

type UserData struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	IdentifyNum    string `json:"identify_num"`
	Online         bool   `json:"online"`
	OS             string `json:"os"`
	Status         int    `json:"status"`
	Avatar         string `json:"avatar"`
	Banner         string `json:"banner"`
	Bot            bool   `json:"bot"`
	MobileVerified bool   `json:"mobile_verified"`
	ClientID       string `json:"client_id"`
	MobilePrefix   string `json:"mobile_prefix"`
	Mobile         string `json:"mobile"`
	InvitedCount   int    `json:"invited_count"`
}
