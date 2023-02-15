package main

import (
	"psm-monitor/misc"
	"psm-monitor/monitor"
	"psm-monitor/net"
	"psm-monitor/slack"

	"fmt"
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
	slack.SendMsg(":zany_face: [APP]", "Monitor now started, components - [PSM, SUN, JST]")
	trackedBlockNumber = net.BlockNumber()
	trackedEvent = make(map[string]func(event *net.Event))
	rand.Seed(time.Now().UnixNano())
}

func track() {
	trackLock.Lock()
	defer trackLock.Unlock()
	latestBlockEvents := net.GetLatestBlockEvents()
	if len(latestBlockEvents) > 0 {
		latestBlockNumber := latestBlockEvents[0].BlockNumber
		if trackedBlockNumber >= latestBlockNumber {
			// current block has already been tracked
			misc.Info("Track task report", fmt.Sprintf("block %d is already tracked", trackedBlockNumber))
		} else {
			for trackedBlockNumber < latestBlockNumber-1 {
				trackedBlockNumber += 1
				events := net.GetBlockEvents(trackedBlockNumber)
				handleEvents(events)
				misc.Info("Track task report", fmt.Sprintf("block %d is missed, has %d events", trackedBlockNumber, len(events)))
			}
			handleEvents(latestBlockEvents)
			trackedBlockNumber = latestBlockNumber
			misc.Info("Track task report", fmt.Sprintf("block %d is latest, has %d events", trackedBlockNumber, len(latestBlockEvents)))
		}
	}
}

func handleEvents(events []*net.Event) {
	for _, event := range events {
		if f, ok := trackedEvent[event.Address]; ok {
			f(event)
		}
	}
}
