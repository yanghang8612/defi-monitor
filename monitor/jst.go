package monitor

import (
    "github.com/robfig/cron"
    "psm-monitor/net"
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

type JST struct {
}

func StartJST(c *cron.Cron, concerned map[string]func(event *net.Event)) {
    jst := &JST{}
    jst.report()

    _ = c.AddFunc("*/9 * * * * ?", jst.check)
    _ = c.AddFunc("0 */10 * * * ?", jst.report)
    _ = c.AddFunc("0 0 */1 * * ?", jst.stats)
}

func (jst *JST) check() {

}

func (jst *JST) report() {

}

func (jst *JST) stats() {

}
