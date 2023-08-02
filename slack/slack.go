package slack

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"psm-monitor/config"
	"psm-monitor/misc"
	"psm-monitor/net"
)

type Message struct {
	Text string `json:"text"`
}

func SendMsg(topic, format string, a ...any) {
	content := format
	if len(a) != 0 {
		content = fmt.Sprintf(format, a...)
	}
	msg := &Message{
		Text: fmt.Sprintf("%s [%s] %s", topic, time.Now().Format("01-02 15:04:05"), content),
	}
	if _, err := net.Post(config.Get().SlackWebhook, msg, checkIfResponseOk); err != nil {
		misc.Warn("Send slack message", fmt.Sprintf("content=\"%s\" res=failed reason=\"%s\"", msg, err.Error()))
	} else {
		misc.Info("Send slack message", fmt.Sprintf("content=\"%s\" res=success", msg))
	}
}

func ReportFee(message string) {
	msg := &Message{
		Text: message,
	}
	if _, err := net.Post(config.Get().FeeSlackWebhook, msg, checkIfResponseOk); err != nil {
		misc.Warn("Send slack message", fmt.Sprintf("content=\"%s\" res=failed reason=\"%s\"", msg, err.Error()))
	} else {
		misc.Info("Send slack message", fmt.Sprintf("content=\"%s\" res=success", msg))
	}
}

func checkIfResponseOk(resBody []byte) error {
	if strings.ContainsAny(string(resBody), "ok") {
		return nil
	}
	return errors.New("Slack response need ok, but got " + string(resBody))
}

func ReportPanic(topic string, err error) {
	SendMsg(":zany_face: [APP]", "Panic happened, doing `%s`, reason `%s`", topic, err.Error())
	// misc.Error("Panic happened", reason)
}
