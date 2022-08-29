package slack

import (
    "errors"
    "psm-monitor/config"
    "psm-monitor/misc"
    "psm-monitor/net"

    "fmt"
    "strings"
)

type Message struct {
    Text string `json:"text"`
}

func SendMsg(topic, format string, a ...any) {
    msg := &Message{
        Text: fmt.Sprintf("[%s] %s", topic, fmt.Sprintf(format, a...)),
    }
    if _, err := net.Post(config.Get().SlackWebhook, msg, checkIfResponseOk); err != nil {
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
    SendMsg("APP", "Panic happened, doing `%s`, reason `%s`", topic, err.Error())
    //misc.Error("Panic happened", reason)
}
