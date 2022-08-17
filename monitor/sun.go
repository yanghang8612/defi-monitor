package monitor

import (
    "encoding/json"
    "psm-monitor/config"
    "psm-monitor/misc"
    "psm-monitor/net"
    "psm-monitor/slack"

    "fmt"
    "math/big"
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

    preUSDDPoolBalance *big.Int
    preUSDTPoolBalance *big.Int
    preA               int64

    // stats params
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

    _ = c.AddFunc("*/9 * * * * ?", misc.WrapLog(sun.check))
    _ = c.AddFunc("0 */10 * * * ?", misc.WrapLog(sun.report))
    _ = c.AddFunc("0 0 */1 * * ?", misc.WrapLog(sun.stats))

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
                msg := fmt.Sprintf("Large exchange `%s => %s` - %s %s sold, %s %s bought, ",
                    soldToken, boughtToken,
                    misc.ToReadableDec(soldAmount, true), soldToken,
                    misc.ToReadableDec(boughtAmount.Neg(boughtAmount), true), boughtToken)
                if diff.Sign() > 0 {
                    msg += fmt.Sprintf(":tada: %s %s lost, %s <!channel>",
                        misc.ToReadableDec(diff, false), boughtToken, misc.FormatTxUrl(event.TransactionHash))
                } else if diff.Sign() < 0 {
                    msg += fmt.Sprintf(":anger: %s %s earned, %s <!channel>",
                        misc.ToReadableDec(diff.Abs(diff), false), boughtToken, misc.FormatTxUrl(event.TransactionHash))
                }
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
            rspData, _ := net.Get("https://apilist.tronscan.org/api/transaction-info?hash=" + event.TransactionHash)
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
                    slack.SendMsg(sun.topic, "Large %s - %s %s, %s <!channel>",
                        event.EventName, tokenName,
                        misc.ToReadableDec(tokenAmount.Neg(tokenAmount), true),
                        misc.FormatTxUrl(event.TransactionHash))
                }
            }
        case "RampA":
            oldA, _ := new(big.Int).SetString(event.Result["old_A"], 10)
            newA, _ := new(big.Int).SetString(event.Result["new_A"], 10)
            slack.SendMsg(sun.topic, "Ramp A from  `%d`  =>  `%d`, %s <!channel>",
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
        slack.SendMsg(s.topic, "Large %s - USDD %s, USDT %s, %s <!channel>",
            event.EventName,
            misc.ToReadableDec(changedLiquidityOfUSDD, true),
            misc.ToReadableDec(changedLiquidityOfUSDT, true),
            misc.FormatTxUrl(event.TransactionHash))
    }
}

func (s *SUN) init() {
    s.sUSDDPoolBalance = s.getPoolUSDDBalance()
    s.sUSDTPoolBalance = s.getPoolUSDTBalance()
    s.preUSDDPoolBalance = s.sUSDDPoolBalance
    s.preUSDTPoolBalance = s.sUSDTPoolBalance
    s.report()
}

func (s *SUN) check() {
    USDDPoolBalance := s.getPoolUSDDBalance()
    USDTPoolBalance := s.getPoolUSDTBalance()

    diffUSDD := big.NewInt(0)
    diffUSDD = diffUSDD.Sub(USDDPoolBalance, s.preUSDDPoolBalance)
    diffUSDT := big.NewInt(0)
    diffUSDT = diffUSDT.Sub(USDTPoolBalance, s.preUSDTPoolBalance)
    if diffUSDT.CmpAbs(big.NewInt(1_000_000)) >= 0 {
        slack.SendMsg("SUN", "Large pool balance change, USDD - %s, USDT - %s <!channel>",
            misc.ToReadableDec(diffUSDD, true),
            misc.ToReadableDec(diffUSDT, true))
    }
    s.preUSDDPoolBalance = USDDPoolBalance
    s.preUSDTPoolBalance = USDTPoolBalance
}

func (s *SUN) report() {
    USDDPoolBalance := s.getPoolUSDDBalance()
    USDTPoolBalance := s.getPoolUSDTBalance()
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
    curA := s.getA()
    slack.SendMsg("SUN", "USDD - %s, USDT - %s, A - `%d`, Ratio - "+Format,
        misc.ToReadableDec(USDDPoolBalance, false),
        misc.ToReadableDec(USDTPoolBalance, false),
        curA,
        USDDFloat64*100/TotalFloat64,
        USDTFloat64*100/TotalFloat64,
        USDDRatio,
        USDTRatio)
    s.preA = curA
}

func (s *SUN) stats() {
    USDDPoolBalance := s.getPoolUSDDBalance()
    USDTPoolBalance := s.getPoolUSDTBalance()
    now := time.Now()
    slack.SendMsg("SUN", "Stats from `%s` ~ `%s`, USDD - %s, USDT - %s",
        s.sTime.Format("15:04"), now.Format("15:04"),
        misc.ToReadableDec(s.sUSDDPoolBalance.Sub(USDDPoolBalance, s.sUSDDPoolBalance), true),
        misc.ToReadableDec(s.sUSDTPoolBalance.Sub(USDTPoolBalance, s.sUSDTPoolBalance), true))
    s.sUSDDPoolBalance = USDDPoolBalance
    s.sUSDTPoolBalance = USDTPoolBalance
    s.sTime = now
}

func (s *SUN) getA() int64 {
    result, err := net.Query(Sun2pool, "A()", "")
    if err != nil {
        slack.ReportPanic(err.Error())
        return s.preA
    }
    return misc.ToBigInt(result).Int64()
}

func (s *SUN) getPoolUSDDBalance() *big.Int {
    if res, err := s.getPoolBalanceOfIndex(0); err == nil {
        return misc.ConvertDec18(res)
    } else {
        return s.preUSDDPoolBalance
    }
}

func (s *SUN) getPoolUSDTBalance() *big.Int {
    if res, err := s.getPoolBalanceOfIndex(1); err == nil {
        return misc.ConvertDec6(res)
    } else {
        return s.preUSDTPoolBalance
    }
}

func (s *SUN) getPoolBalanceOfIndex(i uint) (*big.Int, error) {
    result, err := net.Query(Sun2pool, "balances(uint256)", hexutils.BytesToHex(uint256.NewInt(uint64(i)).PaddedBytes(32)))
    if err != nil {
        slack.ReportPanic(err.Error())
        return big.NewInt(0), err
    }
    return misc.ToBigInt(result), nil
}
