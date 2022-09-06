package monitor

import (
    "psm-monitor/config"
    "psm-monitor/misc"
    "psm-monitor/net"
    "psm-monitor/slack"

    "encoding/json"
    "fmt"
    "math/big"
    "math/rand"
    "strconv"
    "strings"
    "time"

    "github.com/holiman/uint256"
    "github.com/robfig/cron"
    "github.com/status-im/keycard-go/hexutils"
)

const (
    Sun2pool = "TNTfaTpkdd4AQDeqr8SGG7tgdkdjdhbP5c"
)

type SUN struct {
    topic string

    // check values
    cUSDDPoolBalance *big.Int
    cUSDTPoolBalance *big.Int

    // report values
    rUSDDPoolBalance *big.Int
    rUSDTPoolBalance *big.Int
    preA             int64

    // stats values
    sUSDDPoolBalance *big.Int
    sUSDTPoolBalance *big.Int
    sTime            time.Time
}

type oneCoinTx struct {
    TriggerInfo struct {
        Parameter map[string]string
    } `json:"trigger_info"`
}

func StartSUN(c *cron.Cron, concerned map[string]func(event *net.Event)) {
    sun := &SUN{topic: "SUN", sTime: time.Now()}
    sun.init()

    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" */10 * * * ?", misc.WrapLog(sun.check))
    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 0 */1 * * ?", misc.WrapLog(sun.report))
    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 30 */6 * * ?", misc.WrapLog(sun.stats))

    concerned[Sun2pool] = func(event *net.Event) {
        switch event.EventName {
        case "TokenExchange":
            var (
                boughtToken string
                soldToken   string
            )
            boughtAmount, _ := new(big.Int).SetString(event.Result["tokens_bought"], 10)
            soldAmount, _ := new(big.Int).SetString(event.Result["tokens_sold"], 10)
            if strings.Compare(event.Result["sold_id"], "0") == 0 {
                // swap USDD => USDT
                boughtToken = "USDT"
                boughtAmount = misc.ConvertDec6(boughtAmount)
                soldToken = "USDD"
                soldAmount = misc.ConvertDec18(soldAmount)
            } else {
                // swap USDT => USDD
                boughtToken = "USDD"
                boughtAmount = misc.ConvertDec18(boughtAmount)
                soldToken = "USDT"
                soldAmount = misc.ConvertDec6(soldAmount)
            }
            diff := big.NewInt(0)
            diff = diff.Sub(soldAmount, boughtAmount)
            threshold := big.NewInt(config.Get().SUN.SwapThreshold)
            if boughtAmount.Cmp(threshold) > 0 {
                msg := appendWarningIfNeeded(fmt.Sprintf("Large exchange, %s => %s, %s, ",
                    misc.FormatTokenAmt(soldToken, soldAmount, false),
                    misc.FormatTokenAmt(boughtToken, boughtAmount, false),
                    misc.FormatUser(net.GetTxFrom(event.TransactionHash))), boughtToken)
                if diff.Sign() > 0 {
                    msg += fmt.Sprintf("lose %s, slip - `%.2f%%`, ",
                        misc.FormatTokenAmt(boughtToken, diff, false),
                        float64(diff.Uint64())/float64(soldAmount.Uint64())*100)
                } else if diff.Sign() < 0 {
                    msg += fmt.Sprintf("earn %s, slip - `%.2f%%`, ",
                        misc.FormatTokenAmt(boughtToken, diff.Abs(diff), false),
                        float64(diff.Uint64())/float64(soldAmount.Uint64())*100)
                }
                msg += misc.FormatTxUrl(event.TransactionHash) + " <!channel>"
                slack.SendMsg(sun.topic, msg)
            }
        case "AddLiquidity":
            sun.reportLiquidityOperation(event, false)
        case "RemoveLiquidity", "RemoveLiquidityImbalance":
            sun.reportLiquidityOperation(event, true)
        case "RemoveLiquidityOne":
            tokenAmount, _ := new(big.Int).SetString(event.Result["coin_amount"], 10)
            threshold := big.NewInt(config.Get().SUN.LiquidityThreshold)
            tokenName := ""
            rspData, _ := net.Get("https://apilist.tronscan.org/api/transaction-info?hash="+event.TransactionHash, nil)
            var tx oneCoinTx
            if err := json.Unmarshal(rspData, &tx); err == nil {
                if tx.TriggerInfo.Parameter["i"] == "0" {
                    // remove USDD
                    tokenAmount = misc.ConvertDec18(tokenAmount)
                    tokenName = "USDD"
                } else {
                    // remove USDT
                    tokenAmount = misc.ConvertDec6(tokenAmount)
                    tokenName = "USDT"
                }
                if tokenAmount.Cmp(threshold) >= 0 {
                    msg := appendWarningIfNeeded(fmt.Sprintf("Large %s, %s, %s, %s <!channel>",
                        event.EventName,
                        misc.FormatTokenAmt(tokenName, tokenAmount.Neg(tokenAmount), true),
                        misc.FormatUser(net.GetTxFrom(event.TransactionHash)),
                        misc.FormatTxUrl(event.TransactionHash)), tokenName)
                    slack.SendMsg(sun.topic, msg)
                }
            }
        case "RampA":
            oldA, _ := new(big.Int).SetString(event.Result["old_A"], 10)
            newA, _ := new(big.Int).SetString(event.Result["new_A"], 10)
            slack.SendMsg(sun.topic, "Ramp A from  `%d` => `%d`, %s <!channel>",
                oldA, newA, misc.FormatTxUrl(event.TransactionHash))
        }
    }
}

func (s *SUN) reportLiquidityOperation(event *net.Event, isRemove bool) {
    tokenAmounts := strings.Split(event.Result["token_amounts"], "\n")
    changedLiquidityOfUSDD, _ := new(big.Int).SetString(tokenAmounts[0], 10)
    changedLiquidityOfUSDD = misc.ConvertDec18(changedLiquidityOfUSDD)
    if isRemove {
        changedLiquidityOfUSDD = changedLiquidityOfUSDD.Neg(changedLiquidityOfUSDD)
    }
    changedLiquidityOfUSDT, _ := new(big.Int).SetString(tokenAmounts[1], 10)
    changedLiquidityOfUSDT = misc.ConvertDec6(changedLiquidityOfUSDT)
    if isRemove {
        changedLiquidityOfUSDT = changedLiquidityOfUSDT.Neg(changedLiquidityOfUSDT)
    }
    threshold := big.NewInt(config.Get().SUN.LiquidityThreshold)
    if changedLiquidityOfUSDD.CmpAbs(threshold) >= 0 || changedLiquidityOfUSDT.CmpAbs(threshold) >= 0 {
        msg := fmt.Sprintf("Large %s, %s, %s, %s, %s <!channel>",
            event.EventName,
            misc.FormatTokenAmt("USDD", changedLiquidityOfUSDD, true),
            misc.FormatTokenAmt("USDT", changedLiquidityOfUSDT, true),
            misc.FormatUser(net.GetTxFrom(event.TransactionHash)),
            misc.FormatTxUrl(event.TransactionHash))
        if changedLiquidityOfUSDT.Cmp(big.NewInt(0)) < 0 {
            msg = appendWarningIfNeeded(msg, "USDT")
        }
        slack.SendMsg(s.topic, msg)
    }
}

func appendWarningIfNeeded(msg, tokenName string) string {
    if strings.Compare("USDT", tokenName) == 0 {
        // USDT has been token away from pool, we should add exclamation mark
        msg = ":bangbang: " + msg
    }
    return msg
}

func (s *SUN) init() {
    s.cUSDDPoolBalance, s.cUSDTPoolBalance = s.getPoolUSDDBalance(), s.getPoolUSDTBalance()
    s.rUSDDPoolBalance, s.rUSDTPoolBalance = big.NewInt(-1), big.NewInt(-1)
    s.sUSDDPoolBalance, s.sUSDTPoolBalance = s.cUSDDPoolBalance, s.cUSDTPoolBalance
    s.report()
}

func (s *SUN) check() {
    USDDPoolBalance, USDTPoolBalance := s.getPoolUSDDBalance(), s.getPoolUSDTBalance()
    diffUSDD := big.NewInt(0)
    diffUSDD = diffUSDD.Sub(USDDPoolBalance, s.cUSDDPoolBalance)
    diffUSDT := big.NewInt(0)
    diffUSDT = diffUSDT.Sub(USDTPoolBalance, s.cUSDTPoolBalance)
    if diffUSDT.CmpAbs(big.NewInt(config.Get().SUN.ReportThreshold)) >= 0 {
        slack.SendMsg(s.topic, "Large pool balance change in last `10min`, %s, %s <!channel>",
            misc.FormatTokenAmt("USDD", diffUSDD, true),
            misc.FormatTokenAmt("USDT", diffUSDT, true))
    }
    s.cUSDDPoolBalance, s.cUSDTPoolBalance = USDDPoolBalance, USDTPoolBalance
}

func (s *SUN) report() {
    USDDPoolBalance, USDTPoolBalance, curA := s.getPoolUSDDBalance(), s.getPoolUSDTBalance(), s.getA()
    USDDFloat64 := float64(USDDPoolBalance.Uint64())
    USDTFloat64 := float64(USDTPoolBalance.Uint64())
    TotalFloat64 := USDDFloat64 + USDTFloat64
    var (
        USDDRatio float64
        USDTRatio float64
        Format    string
    )
    if USDDPoolBalance.Cmp(USDTPoolBalance) > 0 {
        USDDRatio = USDDFloat64 / USDTFloat64
        USDTRatio = 1.0
        Format = "`%.3f%%` : `%.3f%%` :curly_loop: `%.3f` : `%.0f`"
    } else {
        USDDRatio = 1.0
        USDTRatio = USDTFloat64 / USDDFloat64
        Format = "`%.3f%%` : `%.3f%%` :curly_loop: `%.0f` : `%.3f`"
    }
    slack.SendMsg(s.topic, "State Report, %s, %s, A - `%d`, Ratio - "+Format,
        misc.FormatTokenAmt("USDD", USDDPoolBalance, false),
        misc.FormatTokenAmt("USDT", USDTPoolBalance, false),
        curA,
        USDDFloat64*100/TotalFloat64,
        USDTFloat64*100/TotalFloat64,
        USDDRatio,
        USDTRatio)
    s.rUSDDPoolBalance, s.rUSDTPoolBalance, s.preA = USDDPoolBalance, USDTPoolBalance, curA
}

func (s *SUN) stats() {
    USDDPoolBalance, USDTPoolBalance, now := s.getPoolUSDDBalance(), s.getPoolUSDTBalance(), time.Now()
    slack.SendMsg(s.topic, "Stats Report, from `%s` ~ `%s`, %s, %s",
        s.sTime.Format("15:04"), now.Format("15:04"),
        misc.FormatTokenAmt("USDD", s.sUSDDPoolBalance.Sub(USDDPoolBalance, s.sUSDDPoolBalance), true),
        misc.FormatTokenAmt("USDT", s.sUSDTPoolBalance.Sub(USDTPoolBalance, s.sUSDTPoolBalance), true))
    s.sUSDDPoolBalance, s.sUSDTPoolBalance, s.sTime = USDDPoolBalance, USDTPoolBalance, now
}

func (s *SUN) getA() int64 {
    if result, err := net.Trigger(Sun2pool, "A()", ""); err == nil {
        return misc.ToBigInt(result).Int64()
    } else {
        // if we cannot get current pool A value, return the pre-value
        misc.Warn(s.topic+".getA", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query A value", err.Error()))
        return s.preA
    }
}

func (s *SUN) getPoolUSDDBalance() *big.Int {
    if res, err := s.getPoolBalanceOfIndex(0); err == nil {
        return misc.ConvertDec18(res)
    } else {
        // if we cannot get current USDD pool balance, return the c-value
        misc.Warn(s.topic+".getPoolUSDDBalance", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query USDD pool balance", err.Error()))
        return s.cUSDDPoolBalance
    }
}

func (s *SUN) getPoolUSDTBalance() *big.Int {
    if res, err := s.getPoolBalanceOfIndex(1); err == nil {
        return misc.ConvertDec6(res)
    } else {
        // if we cannot get current USDT pool balance, return the c-value
        misc.Warn(s.topic+".getPoolUSDTBalance", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query USDT pool balance", err.Error()))
        return s.cUSDTPoolBalance
    }
}

func (s *SUN) getPoolBalanceOfIndex(i uint) (*big.Int, error) {
    result, err := net.Trigger(Sun2pool, "balances(uint256)", hexutils.BytesToHex(uint256.NewInt(uint64(i)).PaddedBytes(32)))
    if err != nil {
        return big.NewInt(0), err
    }
    return misc.ToBigInt(result), nil
}
