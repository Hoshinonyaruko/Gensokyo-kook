package handler

import (
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/gookit/event"
	"github.com/idodo/golang-bot/kaihela/api/base"
	event2 "github.com/idodo/golang-bot/kaihela/api/base/event"
	"github.com/idodo/golang-bot/kaihela/api/helper"
	log "github.com/sirupsen/logrus"
)

type ReceiveFrameHandler struct {
}

func (rf *ReceiveFrameHandler) Handle(e event.Event) error {
	log.WithField("event", e).Info("ReceiveFrameHandler receive frame.")
	return nil
}

type GroupEventHandler struct {
}

func (ge *GroupEventHandler) Handle(e event.Event) error {
	log.WithField("event", e).Info("GroupEventHandler receive event.")
	return nil
}

type GroupTextEventHandler struct {
	Token   string
	BaseUrl string
}

func (gteh *GroupTextEventHandler) Handle(e event.Event) error {
	log.WithField("event", fmt.Sprintf("%+v", e.Data())).Info("收到频道内的文字消息.")
	err := func() error {
		if _, ok := e.Data()[base.EventDataFrameKey]; !ok {
			return errors.New("data has no frame field")
		}
		frame := e.Data()[base.EventDataFrameKey].(*event2.FrameMap)
		data, err := sonic.Marshal(frame.Data)
		if err != nil {
			return err
		}
		msgEvent := &event2.MessageKMarkdownEvent{}
		err = sonic.Unmarshal(data, msgEvent)
		log.Infof("Received json event:%+v", msgEvent)
		if err != nil {
			return err
		}
		client := helper.NewApiHelper("/v3/message/create", gteh.Token, gteh.BaseUrl, "", "")
		if msgEvent.Author.Bot {
			log.Info("bot message")
			return nil
		}
		echoData := map[string]string{
			"channel_id": msgEvent.TargetId,
			"content":    "echo:" + msgEvent.KMarkdown.RawContent,
		}
		echoDataByte, err := sonic.Marshal(echoData)
		if err != nil {
			return err
		}
		resp, err := client.SetBody(echoDataByte).Post()
		log.Infof("sent post:%s", client.String())
		if err != nil {
			return err
		}
		log.Infof("resp:%s", string(resp))
		return nil
	}()
	if err != nil {
		log.WithError(err).Error("GroupTextEventHandler err")
	}

	return nil
}
