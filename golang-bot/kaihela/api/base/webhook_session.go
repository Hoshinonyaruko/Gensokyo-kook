package base

import (
	"errors"

	"github.com/bytedance/sonic"
	event2 "github.com/idodo/golang-bot/kaihela/api/base/event"
	"github.com/idodo/golang-bot/kaihela/api/helper"
	log "github.com/sirupsen/logrus"
)

type WebhookSession struct {
	Session
	EncryptKey  string
	VerifyToken string
}

func NewWebhookSession(encryptKey, verityToken string, compress int) *WebhookSession {
	session := &WebhookSession{}
	if encryptKey != "" {
		session.EncryptKey = encryptKey
	}
	if verityToken != "" {
		session.VerifyToken = verityToken
	}
	session.Compressed = compress
	session.Session.ProcessDataHandler = session.ProcessData
	session.Session.ReceiveFrameHandler = session.ReceiveFrameHandler
	return session
}

func (s *WebhookSession) ProcessData(data []byte) (data2 []byte, err error) {
	// 如果有加密，则对数据进行解密
	if s.EncryptKey == "" {
		data2 = data
		return
	}
	jdata, err := sonic.Get(data)
	if err != nil {
		return nil, err
	}
	if !jdata.Get("encrypt").Exists() { // 简化布尔值比较
		log.Error("Encrypt_Data Not Exist", string(data))
		err = errors.New("Encrypt_Data Not Exist")
		return
	}
	encryptText, err := jdata.Get("encrypt").String()
	if err != nil {
		log.Error(err)
		return
	}
	//log.Tracef("encryptText:%s", encryptText)
	err, plainByte := helper.DecryptData(encryptText, s.EncryptKey)
	if err != nil {
		log.WithError(err).Error("DecryptData failed")
		return
	}
	return plainByte, nil
}

func (s *WebhookSession) ReceiveFrameHandler(frame *event2.FrameMap) ([]byte, error) {
	if s.VerifyToken != "" {
		gotVerifyToken := ""
		if v, ok := frame.Data["verify_token"]; ok {
			gotVerifyToken = v.(string)
		}
		if gotVerifyToken != s.VerifyToken {
			log.WithField("gotVerifyToken", gotVerifyToken).Error("gotVerifyToken Error")
			return nil, errors.New("VerifyToken error")
		}
	}
	retData := make(map[string]interface{})
	if frame.SignalType == event2.SIG_EVENT {
		if _, ok := frame.Data["type"]; ok {
			if challenge, ok := frame.Data["challenge"]; ok {
				retData["challenge"] = challenge
			}
		}
	}
	s.Session.ReceiveFrame(frame)
	retByte, err := sonic.Marshal(retData)
	if err != nil {
		log.WithError(err).Error("marshal retData error")
		return nil, err
	}
	return retByte, nil

}

func (s *WebhookSession) SendData(data []byte) error {
	return errors.New("webhook不能主动发消息给服务端")
}
