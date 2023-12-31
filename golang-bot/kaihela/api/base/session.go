package base

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/bytedance/sonic"
	"github.com/gookit/event"
	event2 "github.com/idodo/golang-bot/kaihela/api/base/event"
	log "github.com/sirupsen/logrus"
)

const EventReceiveFrame = "EVENT-GLOBAL-RECEIVE_FRAME"
const EventDataFrameKey = "frame"
const EventDataSessionKey = "session"

var global_s int64

type Session struct {
	Compressed          int
	ReceiveFrameHandler func(frame *event2.FrameMap) ([]byte, error)
	ProcessDataHandler  func(data []byte) ([]byte, error)
	EventSyncHandle     bool
}

func (s *Session) On(message string, handler event.Listener) {
	event.On(message, handler)
}
func (s *Session) Trigger(eventName string, params event.M) {
	if s.EventSyncHandle {
		event.Trigger(eventName, params)
	} else {
		event.AsyncFire(event.NewBasic(eventName, params))
	}
}
func (s *Session) ReceiveData(data []byte) ([]byte, error) {
	if s.Compressed == 1 {
		b := bytes.NewReader(data)
		r, err := zlib.NewReader(b)
		if err != nil {
			return nil, err
		}

		data, err = io.ReadAll(r)
		if err != nil {
			log.Error(err)
			return nil, err
		}

	}
	_, err := sonic.Get(data)
	if err != nil {
		log.Error("Json Unmarshal err.", err)
		return nil, err
	}
	if s.ProcessDataHandler != nil {
		data, err = s.ProcessDataHandler(data)
		if err != nil {
			log.WithError(err).Error("ProcessDataHandler")
			return nil, err
		}
	}
	frame := event2.ParseFrameMapByData(data)
	log.WithField("frame", frame).Info("Receive frame from server")
	if frame != nil {
		// 更新 global_s 的值
		atomic.StoreInt64(&global_s, frame.SerialNumber)
		if s.ReceiveFrameHandler != nil {
			return s.ReceiveFrameHandler(frame)
		} else {
			return s.ReceiveFrame(frame)
		}
	} else {
		log.Warnf("数据不是合法的frame:%s", string(data))
	}
	return nil, nil

}

func (s *Session) ReceiveFrame(frame *event2.FrameMap) ([]byte, error) {
	event.Trigger(EventReceiveFrame, map[string]interface{}{"frame": frame})
	if frame.SignalType == event2.SIG_EVENT {
		eventType := frame.Data["type"]
		channelType := frame.Data["channel_type"].(string)
		if eventType != "" {
			name := fmt.Sprintf("%s_%d", channelType, int64(eventType.(float64)))
			fireEvent := event.NewBasic(name, map[string]interface{}{EventDataFrameKey: frame, EventDataSessionKey: s})
			if s.EventSyncHandle {
				event.Trigger(fireEvent.Name(), fireEvent.Data())
			} else {
				event.AsyncFire(fireEvent)
			}
		}
	}
	return nil, nil
}

func GetGlobalS() int64 {
	return atomic.LoadInt64(&global_s)
}
