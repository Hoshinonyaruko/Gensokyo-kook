package main

import (
	"bytes"
	"compress/zlib"
	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
	event2 "golang-bot/kaihela/api/base/event"
	"io"
	"net/http"
	"testing"
)

func TestSendChallenge(t *testing.T) {
	signal := event2.NewChallengeEventSignal("xxxx", "yyyy")
	data, err := sonic.Marshal(signal)
	var buf bytes.Buffer
	g := zlib.NewWriter(&buf)
	if _, err = g.Write(data); err != nil {
		log.Error(err)
		return
	}
	g.Close()
	client := http.Client{}
	req, err := http.NewRequest("POST", "http://127.0.0.1:8080", &buf)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Encoding", "compress/zlib")
	resp, err := client.Do(req)
	respData, err := io.ReadAll(resp.Body)

	t.Log("respData", string(respData))
}
