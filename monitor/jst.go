package monitor

import (
	"math/big"
	"math/rand"
	"strconv"

	"psm-monitor/config"
	"psm-monitor/misc"
	"psm-monitor/net"
	"psm-monitor/slack"

	"github.com/robfig/cron"
)

const (
	jTRX        = "TE2RzoSV3wFK99w6J9UnnZ4vLfXYoxvRwP"
	jUSDD       = "TX7kybeP6UwTBRHLNPYmswFESHfyjm9bAS"
	jUSDT       = "TXJgMdjVX5dKiQaUi9QobwNxtSQaFqccvd"
	jSUN        = "TPXDpkg9e3eZzxqxAUyke9S4z4pGJBJw9e"
	jBTT        = "TUaUHU9Dy8x5yNi1pKnFYqHWojot61Jfto"
	jNFT        = "TFpPyDCKvNFgos3g3WVsAqMrdqhB81JXHE"
	jJST        = "TWQhCXaWz4eHK4Kd1ErSDHjMFPoPc9czts"
	jWIN        = "TRg6MnpsFXc82ymUPgf5qbj59ibxiEDWvv"
	jUSDJ       = "TL5x9MtSnDy537FXKx53yAaHRRNdg9TkkA"
	jUSDC       = "TNSBA6KvSvMoTqQcEgpVK7VhHT3z7wifxy"
	jTUSD       = "TSXv71Fy5XdL3Rh2QfBoUu3NAaM4sMif8R"
	jBTC        = "TLeEu311Cbw63BcmMHDgDLu7fnk9fqGcqT"
	jETH        = "TR7BUFRQeq1w5jAZf1FKx85SHuX6PfMqsV"
	jWBTT       = "TUY54PVeH6WCcYCd6ZXXoBDsHytN9V5PXt"
	jSUNOLD     = "TGBr8uh9jBVHJhhkwSJvQN2ZAKzVkxDmno"
	jController = "TGjYzgCyPobsNS9n6WcbdLVR9dH7mWqFx7"
)

type market struct {
	symbol   string
	decimals uint
}

type JST struct {
	topic string

	markets map[string]market
}

func StartJST(c *cron.Cron, concerned map[string]func(event *net.Event)) {
	jst := &JST{topic: ":justlend: [JST]", markets: make(map[string]market)}
	jst.markets[jTRX] = market{symbol: "TRX", decimals: 8}
	jst.markets[jUSDD] = market{symbol: "USDD", decimals: 18}
	jst.markets[jUSDT] = market{symbol: "USDT", decimals: 6}
	jst.markets[jSUN] = market{symbol: "SUN", decimals: 18}
	jst.markets[jBTT] = market{symbol: "BTT", decimals: 18}
	jst.markets[jNFT] = market{symbol: "NFT", decimals: 6}
	jst.markets[jJST] = market{symbol: "JST", decimals: 18}
	jst.markets[jWIN] = market{symbol: "WIN", decimals: 6}
	jst.markets[jUSDJ] = market{symbol: "USDJ", decimals: 18}
	jst.markets[jUSDC] = market{symbol: "USDC", decimals: 6}
	jst.markets[jTUSD] = market{symbol: "TUSD", decimals: 18}
	jst.markets[jBTC] = market{symbol: "BTC", decimals: 8}
	jst.markets[jETH] = market{symbol: "ETH", decimals: 18}
	jst.init()

	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" */10 * * * ?", misc.WrapLog(jst.check))
	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 0 */1 * * ?", misc.WrapLog(jst.report))
	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 30 */6 * * ?", misc.WrapLog(jst.stats))

	concerned[jUSDD] = jst.handleStableCoin
	concerned[jUSDT] = jst.handleStableCoin
	concerned[jUSDJ] = jst.handleStableCoin
	concerned[jUSDC] = jst.handleStableCoin
	concerned[jTUSD] = jst.handleStableCoin
}

func (j *JST) handleStableCoin(event *net.Event) {
	jMarket := j.markets[event.Address]
	threshold := big.NewInt(config.Get().JST.StableThreshold)
	switch event.EventName {
	case "Borrow":
		borrowAmount, _ := new(big.Int).SetString(event.Result["borrowAmount"], 10)
		borrowAmount = misc.ConvertDecN(borrowAmount, jMarket.decimals)
		borrower := event.Result["borrower"]
		if borrowAmount.Cmp(threshold) >= 0 {
			slack.SendMsg(j.topic, "Large %s, %s, %s, %s <!channel>",
				event.EventName,
				misc.FormatTokenAmt(jMarket.symbol, borrowAmount, false),
				misc.FormatUser(borrower),
				misc.FormatTxUrl(event.TransactionHash))
		}
	case "Redeem":
		redeemAmount, _ := new(big.Int).SetString(event.Result["redeemAmount"], 10)
		redeemAmount = misc.ConvertDecN(redeemAmount, jMarket.decimals)
		redeemer := event.Result["redeemer"]
		if redeemAmount.Cmp(threshold) >= 0 {
			slack.SendMsg(j.topic, "Large %s, %s, %s, %s <!channel>",
				event.EventName,
				misc.FormatTokenAmt(jMarket.symbol, redeemAmount, false),
				misc.FormatUser(redeemer),
				misc.FormatTxUrl(event.TransactionHash))
		}
	}
}

func (j *JST) init() {

}

func (j *JST) check() {

}

func (j *JST) report() {

}

func (j *JST) stats() {

}
