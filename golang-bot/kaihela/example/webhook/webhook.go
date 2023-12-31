package main

import (
	"fmt"
	"github.com/idodo/golang-bot/kaihela/api/base"
	"github.com/idodo/golang-bot/kaihela/example/conf"
	"github.com/idodo/golang-bot/kaihela/example/handler"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

func main() {
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{})
	log.SetLevel(log.InfoLevel)

	session := base.NewWebhookSession(conf.EncryptKey, conf.VerifyToken, 1)
	session.On(base.EventReceiveFrame, &handler.ReceiveFrameHandler{})
	session.On("GROUP*", &handler.GroupEventHandler{})
	session.On("GROUP_9", &handler.GroupTextEventHandler{Token: conf.Token, BaseUrl: conf.BaseUrl})
	http.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "application/json")
		defer req.Body.Close()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.WithError(err).Error("Read req body error")
			return
		}
		err, resData := session.ReceiveData(body)
		if err != nil {
			log.WithError(err).Error("handle req err")
		}
		resp.Write(resData)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", conf.HTTPServerPort), nil))

}
