package base

type SystemInterface interface {
	ReqGateWay() (string, error)
	ConnectWebsocket(gateway string) error
	SendData(data []byte) error
	//ReceiveData(data []byte) (error, []byte)
	SaveSessionId(sessionId string) error
}
