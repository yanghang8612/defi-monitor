package main

import (
    "psm-monitor/misc"
    "psm-monitor/monitor"
    "psm-monitor/net"
    "psm-monitor/slack"

    "math/rand"
    "sync"
    "time"

    "github.com/robfig/cron"
)

var (
    trackedBlockNumber uint64
    trackedEvent       map[string]func(event *net.Event)
    trackLock          sync.RWMutex
)

func main() {
    initApp()

    c := cron.New()
    monitor.StartPSM(c, trackedEvent)
    monitor.StartSUN(c, trackedEvent)
    monitor.StartJST(c, trackedEvent)
    _ = c.AddFunc("*/3 * * * * ?", misc.WrapLog(track))
    c.Start()

    defer c.Stop()
    select {}
}

func initApp() {
    slack.SendMsg("APP", "Monitor now started, components - [PSM, SUN, JST]")
    trackedBlockNumber = net.BlockNumber() - 1
    trackedEvent = make(map[string]func(event *net.Event))
    rand.Seed(time.Now().UnixNano())
}

func track() {
    trackLock.RLock()
    defer trackLock.RUnlock()
    events := net.GetBlockEvents(trackedBlockNumber + 1)
    for _, event := range events {
        if f, ok := trackedEvent[event.Address]; ok {
            f(&event)
        }
    }
    trackedBlockNumber += 1
}
