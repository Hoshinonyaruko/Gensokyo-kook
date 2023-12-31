package base

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/bytedance/sonic"
	event2 "github.com/idodo/golang-bot/kaihela/api/base/event"
	helper "github.com/idodo/golang-bot/kaihela/api/helper"
	"github.com/looplab/fsm"
	cron "github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

type State struct {
	Name string
	Code int
}

const (
	// 默认开始状态
	StatusStart = "start"
	// 初始状态
	StatusInit = "init"
	// 网关已获取
	StatusGateway = "gateway"
	// ws已经连接，等待hello包
	StatusWSConnected = "ws_connected"
	//已连接状态
	StatusConnected = "connected"
	//resume
	StatusRetry = "retry"
)

const (
	EventEnterPrefix           = "enter_"
	EventStart                 = "fsmStart"
	EventGotGateway            = "getGateWay"
	EventWsConnected           = "wsConnect"
	EventWsConnectFail         = "wsConnectFail"
	EventHelloReceived         = "helloReceived"
	EventHelloFail             = "helloFail"
	EventHelloGatewayErrFail   = "helloGatewayErrFail"
	EventPongReceived          = "pongReceived"
	EventHeartbeatTimeout      = "heartbeatTimeout"
	EventRetryHeartbeatTimeout = "retryHeartbeatTimeout"
	EventResumeReceivedOk      = "ResumeReceived"
)

const (
	NO_RETRY      = -1
	RETRY_INFINIT = 0
)

type StatusParam struct {
	StartTime  int
	MaxTime    int
	FirstDelay int
	Retry      int
	MaxRetry   int
}

/**                                                _________________
 *       获取gateWay     连接ws          收到hello |    心跳超时    |
 *             |           |                |      |      |         |
 *             v           v                v      |      V         |
 *      INIT  --> GATEWAY -->  WS_CONNECTED --> CONNECTED --> RETRY |
 *       ^        |   ^             |                  ^_______|    |
 *       |        |   |_____________|__________________________|    |
 *       |        |                 |                          |    |
 *       |________|_________________|__________________________|____|
 *
 */
type StateSession struct {
	Session
	SessionId string
	//Status           string
	GateWay      string
	Timeout      int
	RecvQueue    chan *event2.FrameMap
	MaxSn        int64
	FSM          *fsm.FSM
	NetworkProxy SystemInterface

	StatusParams    map[string]*StatusParam
	HeartBeatCron   *cron.Cron
	LastPongAt      time.Time
	LastPingAt      time.Time
	PongTimeoutChan chan time.Time
}

func NewStateSession(gateway string, compressed int) *StateSession {
	s := &StateSession{}
	s.StatusParams = map[string]*StatusParam{
		StatusInit:        {StartTime: 0, MaxTime: 60, FirstDelay: 1, MaxRetry: RETRY_INFINIT},
		StatusGateway:     {StartTime: 1, MaxTime: 32, FirstDelay: 2, MaxRetry: 2},
		StatusWSConnected: {StartTime: 6, MaxTime: 0, FirstDelay: 0, MaxRetry: NO_RETRY},
		StatusConnected:   {StartTime: 30, MaxTime: 30, FirstDelay: 0, MaxRetry: NO_RETRY},
		StatusRetry:       {StartTime: 0, MaxTime: 8, FirstDelay: 4, MaxRetry: 2},
	}
	s.Session.ReceiveFrameHandler = s.ReceiveFrameHandler
	s.Compressed = compressed
	s.GateWay = gateway
	s.RecvQueue = make(chan *event2.FrameMap)
	//
	s.FSM = fsm.NewFSM(
		StatusStart,
		fsm.Events{
			{Name: EventStart, Src: []string{StatusStart}, Dst: StatusInit},
			{Name: EventGotGateway, Src: []string{StatusInit}, Dst: StatusGateway},
			{Name: EventWsConnected, Src: []string{StatusGateway}, Dst: StatusWSConnected},
			{Name: EventWsConnectFail, Src: []string{StatusGateway}, Dst: StatusInit},
			{Name: EventHelloReceived, Src: []string{StatusWSConnected}, Dst: StatusConnected},
			{Name: EventHelloFail, Src: []string{StatusWSConnected}, Dst: StatusGateway},
			{Name: EventHelloGatewayErrFail, Src: []string{StatusWSConnected}, Dst: StatusInit},                //hello收到特定错误码：40100, 40101, 40102, 40103等
			{Name: EventPongReceived, Src: []string{StatusConnected, StatusWSConnected}, Dst: StatusConnected}, //??StatusWSConnected
			{Name: EventHeartbeatTimeout, Src: []string{StatusConnected}, Dst: StatusRetry},
			{Name: EventRetryHeartbeatTimeout, Src: []string{StatusRetry}, Dst: StatusGateway},
			{Name: EventResumeReceivedOk, Src: []string{StatusWSConnected, StatusConnected}, Dst: StatusConnected},
		},
		fsm.Callbacks{
			"enter_state": func(_ context.Context, e *fsm.Event) {
				log.WithField("from", e.Src).WithField("to", e.Dst).Info("state change")
			},
			EventEnterPrefix + StatusInit: func(_ context.Context, e *fsm.Event) {
				s.Retry(e, func() error { return s.GetGateway() }, nil)
			},
			EventEnterPrefix + StatusGateway: func(_ context.Context, e *fsm.Event) {
				s.Retry(e, func() error { return s.WsConnect() }, func() error { return s.wsConnectFail() })
			},
			EventEnterPrefix + StatusWSConnected: func(_ context.Context, e *fsm.Event) {

			},
			EventEnterPrefix + StatusConnected: func(_ context.Context, e *fsm.Event) {
				s.HeartBeatCron.Start()
				s.StartCheckHeartbeat()
			},
			EventEnterPrefix + StatusRetry: func(_ context.Context, e *fsm.Event) {
				s.Retry(e, func() error { s.SendHeartBeat(); return errors.New("just for continue to send heartbeat") }, nil)
			},
		},
	)

	s.HeartBeatCron = cron.New()
	interval := s.StatusParams[StatusConnected].MaxTime
	s.HeartBeatCron.AddFunc(fmt.Sprintf("@every %ds", interval), func() {
		s.SendHeartBeat()
	})
	s.PongTimeoutChan = make(chan time.Time)
	return s
}

func (s *StateSession) Start() {
	if s.GateWay == "" {
		s.FSM.SetState(StatusInit)
		s.Retry(nil, func() error { return s.GetGateway() }, nil)

	} else {
		s.FSM.SetState(StatusGateway)
		s.Retry(nil, func() error { return s.WsConnect() }, func() error { return s.wsConnectFail() })
	}
	s.StartProcessEvent()
}

func (s *StateSession) GetGateway() error {
	log.Info("state", "getGateway")
	s.Trigger("status_getGateWay", nil)
	gateWay, err := s.NetworkProxy.ReqGateWay()

	if err == nil && gateWay != "" {
		s.getGateWayOK(gateWay)
	} else {
		log.Error("getGateway error", err)
		return errors.New("reqGateWay error")
	}
	return nil
}

func (s *StateSession) Retry(e *fsm.Event, handler func() error, errHandler func() error) {
	log.Infof("Retry handler:%s", helper.GetFunctionName(handler))
	startTime := s.StatusParams[s.FSM.Current()].StartTime
	maxTime := s.StatusParams[s.FSM.Current()].MaxTime
	maxRetry := s.StatusParams[s.FSM.Current()].MaxRetry
	firstDelay := s.StatusParams[s.FSM.Current()].FirstDelay
	if e != nil {
		if len(e.Args) > 0 {
			if param, ok := e.Args[0].(*StatusParam); ok {
				if param.StartTime > 0 {
					startTime = param.StartTime
				}
				if param.MaxTime > 0 {
					maxTime = param.MaxTime
				}
				if param.FirstDelay > 0 {
					firstDelay = param.FirstDelay
				}
				if param.MaxRetry != 0 {
					maxRetry = param.MaxRetry
				}

			}
		}
	}
	//等待start时间开始
	time.Sleep(time.Second * time.Duration(startTime))

	//不用指数重试
	if maxRetry == NO_RETRY {
		err := handler()
		if err != nil {
			log.WithError(err).Infof("Retry function error: %s", helper.GetFunctionName(handler))
			if errHandler != nil {
				errHandler()
			}
		}
		return
	}

	//指数重试
	err := retry.Do(
		handler,
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(time.Second*time.Duration(firstDelay)),
		retry.MaxDelay(time.Second*time.Duration(maxTime)),
		retry.Attempts(uint(maxRetry)),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Infof("try %d times call function %s", n, getFunctionName(handler))
		}),
	)
	if err != nil && errHandler != nil {
		errHandler()
	}
}

// Helper function to get function name
func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func (s *StateSession) getGateWayOK(gateWay string) {
	log.WithField("gateway", gateWay).Info("GetGatewayOk")
	s.GateWay = gateWay
	s.FSM.Event(context.Background(), EventGotGateway)
}

// WsConnect : Try to websocket connect
func (s *StateSession) WsConnect() error {
	return s.NetworkProxy.ConnectWebsocket(s.GateWay)
}

func (s *StateSession) wsConnectFail() error {
	log.Warn("wsConnectFail")
	s.FSM.Event(context.Background(), EventWsConnectFail)
	return nil
}

func (s *StateSession) wsConnectOk() {
	log.Info("wsConnectOk")
	s.FSM.Event(context.Background(), EventWsConnected)
}

// func (s *StateSession) helloFail() {
// 	log.Info("helloFail")
// 	s.FSM.Event(context.Background(), EventHelloFail)
// }

func (s *StateSession) receiveHello(frameMap *event2.FrameMap) {
	code := 40100
	if _code, ok := frameMap.Data["code"]; ok {
		code = int(_code.(float64))
	}
	if code == 0 {
		s.LastPongAt = time.Now()
		log.Info("receiveHello")
		s.SaveSessionId(frameMap.Data["sessionId"].(string))
		s.FSM.Event(context.Background(), EventHelloReceived)
	} else {
		log.Warn("connectFailed", code)
		if helper.SliceContains([]int{40100, 40101, 40102, 40103}, code) {

			s.FSM.Event(context.Background(), EventHelloGatewayErrFail, &StatusParam{StartTime: 6})
		}
	}
}

func (s *StateSession) SaveSessionId(sessionId string) {
	s.SessionId = sessionId
	s.NetworkProxy.SaveSessionId(sessionId)
}

func (s *StateSession) StartProcessEvent() {
	go func() {
		for frame := range s.RecvQueue {
			s.ReceiveFrame(frame)
		}
	}()
}

func (s *StateSession) ReceiveFrameHandler(frame *event2.FrameMap) ([]byte, error) {
	switch frame.SignalType {
	case event2.SIG_EVENT:
		{
			if s.FSM.Current() == StatusConnected {
				if frame.SerialNumber > s.MaxSn {
					s.MaxSn = frame.SerialNumber
				}
				s.RecvQueue <- frame
			}
		}
	case event2.SIG_HELLO:
		{
			s.receiveHello(frame)
		}
	case event2.SIG_PONG:
		{
			s.receivePong(frame)
		}
	case event2.SIG_RESUME_ACK:
		{
			s.ResumeOk()
		}
	case event2.SIG_RECONNECT:
		{
			s.Reconnect()
		}

	}
	return nil, nil

}

func (s *StateSession) SendHeartBeat() error {
	pingFrame := event2.NewPingFrame(s.MaxSn)
	if s.NetworkProxy != nil {
		data, err := sonic.Marshal(pingFrame)
		if err != nil {
			log.WithError(err).Error("sendHeartBeat unmarsal fail")
			return err
		}
		s.LastPingAt = time.Now()
		log.WithField("ping", string(data)).Info("Send Ping")
		err = s.NetworkProxy.SendData(data)
		if err != nil {
			return err
		} else {
			s.PongTimeoutChan <- s.LastPingAt.Add(time.Duration(s.Timeout) * time.Second)
		}
	}
	return nil
}

func (s *StateSession) StartHeartbeat() error {

	s.HeartBeatCron.Start()
	return nil
}

func (s *StateSession) RetryHeartbeat() error {
	return s.SendHeartBeat()
}

func (s *StateSession) receivePong(frame *event2.FrameMap) {
	log.Infof("receivePong %+v", frame)
	s.FSM.Event(context.Background(), EventPongReceived)
	s.LastPongAt = time.Now()
}

func (s *StateSession) StartCheckHeartbeat() {
	log.Info("Start heartBeatTimeout check")
	go func() {
		for pongTimeoutAt := range s.PongTimeoutChan {
			log.WithField("pongTimeoutAt", pongTimeoutAt).Info("check pong receive timeout")
			if s.FSM.Current() != StatusConnected && s.FSM.Current() != StatusRetry {
				continue
			}
			if time.Now().Before(pongTimeoutAt) { // 还没有到的timeout检查时间点
				time.Sleep(time.Until(pongTimeoutAt))
				// 最后收到Pong时间比（约定检查时间-最大过期时间）早，表示在过去的约定的过期时间内及之后没有收到Pong
				if s.LastPongAt.Before(pongTimeoutAt.Add(-time.Duration(s.Timeout) * time.Second)) {
					log.Infof("Pong not received before:%s", pongTimeoutAt)
					if s.FSM.Current() == StatusConnected {
						err := s.FSM.Event(context.Background(), EventHeartbeatTimeout)
						if err == nil {
							s.HeartBeatCron.Stop()
						}
					}

					if s.FSM.Current() == StatusRetry {
						err := s.FSM.Event(context.Background(), EventRetryHeartbeatTimeout)
						if err == nil {
							log.Infof("403 error:%v", err)
						}
					}
				}
			}
		}
	}()
}

func (s *StateSession) ResumeOk() {
	s.Trigger("status_resumeOk", nil)
	log.Info("resumeOk")
	if s.FSM.Current() != StatusConnected {
		s.FSM.Event(context.Background(), EventResumeReceivedOk)
	}
}

func (s *StateSession) Reconnect() {
	s.Trigger("status_reconnect", nil)
	log.Info("reconnect")
	s.HeartBeatCron.Stop()
	s.GateWay = ""
	s.RecvQueue = make(chan *event2.FrameMap)
	s.MaxSn = 0
	s.SaveSessionId("")
	s.FSM.SetState(StatusInit)
	s.Retry(nil, func() error { return s.GetGateway() }, nil)
}
