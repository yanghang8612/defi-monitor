package slack

import (
    "fmt"
)

type Message struct {
    Text string `json:"text"`
}

func SendMsg(topic, format string, a ...any) {
    msg := &Message{
        Text: fmt.Sprintf("[%s] %s", topic, fmt.Sprintf(format, a...)),
    }
    fmt.Println(msg)
    //_, _ = net.Post(config.Get().SlackWebhook, msg)
}

func ReportPanic(reason string) {
    SendMsg("APP", reason)
}
