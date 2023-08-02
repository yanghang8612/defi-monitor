package monitor

import (
	"fmt"
	"time"

	"github.com/robfig/cron"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"psm-monitor/misc"
	"psm-monitor/net"
	"psm-monitor/slack"
)

type Record struct {
	ID            uint `gorm:"primaryKey"`
	TrackedAt     time.Time
	TronLowPrice  float64
	TronHighPrice float64
	EthLowPrice   float64
	EthHighPrice  float64
}

var appDB *gorm.DB

func StartTrackFee(c *cron.Cron) {
	_ = c.AddFunc("0 */1 * * * ?", misc.WrapLog(track))
	_ = c.AddFunc("30 0 2 * * ?", misc.WrapLog(report))

	db, err := gorm.Open(sqlite.Open("monitor.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&Record{})

	appDB = db
}

func track() {
	trxPrice := net.GetPrice("TRX")
	ethPrice := net.GetPrice("ETH")
	gasPrice := net.GetGasPrice()
	energyPrice, factor := net.GetEnergyPriceAndFactor()

	tronLowPrice := trxPrice * energyPrice * (1 + factor/1e4) * 14650 / 1e6
	tronHighPrice := trxPrice * energyPrice * (1 + factor/1e4) * 29650 / 1e6
	ethLowPrice := ethPrice * float64(gasPrice) * 41309 / 1e9
	ethHighPrice := ethPrice * float64(gasPrice) * 63209 / 1e9

	appDB.Create(&Record{TrackedAt: time.Now(), TronLowPrice: tronLowPrice, TronHighPrice: tronHighPrice, EthLowPrice: ethLowPrice, EthHighPrice: ethHighPrice})
}

func report() {
	now := time.Now()

	var dayAvgs [4]float64
	preDay := now.AddDate(0, 0, -1)
	appDB.Model(&Record{}).
		Select("AVG(tron_low_price), AVG(tron_high_price), AVG(eth_low_price), AVG(eth_high_price)").
		Where("tracked_at BETWEEN ? AND ?", preDay, now).Row().Scan(&dayAvgs[0], &dayAvgs[1], &dayAvgs[2], &dayAvgs[3])

	var weekAvgs [4]float64
	preWeek := now.AddDate(0, 0, -7)
	appDB.Model(&Record{}).
		Select("AVG(tron_low_price), AVG(tron_high_price), AVG(eth_low_price), AVG(eth_high_price)").
		Where("tracked_at BETWEEN ? AND ?", preWeek, now).Row().Scan(&weekAvgs[0], &weekAvgs[1], &weekAvgs[2], &weekAvgs[3])

	slackMessage := ""
	slackMessage += fmt.Sprintf("> USDT 日均手续费: `%.2f$` - `%.2f$` @TRON / `%.2f$` - `%.2f$` @ETH\n", dayAvgs[0], dayAvgs[1], dayAvgs[2], dayAvgs[3])
	slackMessage += fmt.Sprintf("> USDT 周均手续费: `%.2f$` - `%.2f$` @TRON / `%.2f$` - `%.2f$` @ETH\n", weekAvgs[0], weekAvgs[1], weekAvgs[2], weekAvgs[3])

	slack.ReportFee(slackMessage)
}
