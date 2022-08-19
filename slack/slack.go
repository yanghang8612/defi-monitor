package slack

import (
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
    res, err := net.Post(config.Get().SlackWebhook, msg)
    if err != nil {
        misc.Warn("Send slack message", fmt.Sprintf("content=\"%s\" res=failed reason=\"%s\"", msg, err.Error()))
    } else if strings.Compare("true", string(res)) != 0 {
        misc.Warn("Send slack message", fmt.Sprintf("content=\"%s\" res=failed reason=\"slack retruned false\"", msg))
    } else {
        misc.Info("Send slack message", fmt.Sprintf("content=\"%s\" res=success", msg))
    }
}

func ReportPanic(reason string) {
    //SendMsg("APP", reason)
    misc.Error("Panic happened", reason)
}
