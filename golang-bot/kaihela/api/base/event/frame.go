package event

import (
	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
)

type Frame struct {
	SignalType int32 `json:"s"`
}
type FrameMap struct {
	SignalType   int32                  `json:"s"`
	Data         map[string]interface{} `json:"d"`
	SerialNumber int64                  `json:"sn"`
}

func ParseFrameMapByData(data []byte) *FrameMap {

	frame := &FrameMap{}
	err := sonic.Unmarshal(data, frame)
	if err != nil {
		log.Error("data unmarshal err", err)
		return nil
	}
	return frame
}

func NewPingFrame(sn int64) *FrameMap {
	frame := &FrameMap{}
	frame.SerialNumber = sn
	frame.SignalType = SIG_PING
	//frame.Data = make(map[string]interface{})
	return frame
}
