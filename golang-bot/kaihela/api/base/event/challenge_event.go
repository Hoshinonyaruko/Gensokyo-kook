package event

type ChallengeEventSignal struct {
	Frame
	ChallengeData `json:"d"`
}
type ChallengeData struct {
	Type        int    `json:"type"`
	ChannelType string `json:"channel_type"`
	Challenge   string `json:"challenge"`
	VerifyToken string `json:"verify_token"`
}

func NewChallengeEventSignal(challenge, verifyToken string) *ChallengeEventSignal {
	return &ChallengeEventSignal{
		Frame:         Frame{SignalType: SIG_EVENT},
		ChallengeData: ChallengeData{Type: 255, ChannelType: "WEBHOOK_CHALLENGE", Challenge: challenge, VerifyToken: verifyToken},
	}
}
