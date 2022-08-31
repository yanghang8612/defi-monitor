package monitor

import (
    "fmt"
    "psm-monitor/config"
    "psm-monitor/misc"
    "psm-monitor/net"
    "psm-monitor/slack"
    "strings"

    "math/big"
    "math/rand"
    "strconv"
    "time"

    "github.com/robfig/cron"
)

const (
    USDD_DaiJoin = "TMgSSHn8APyUVViqXxtveqFEB7mBBeGqNP"
    USDT         = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
    USDT_GemJoin = "TMn5WeW8a8KH9o8rBQux4RCgckD2SuMZmS"
    USDT_PSM     = "TM9gWuCdFGNMiT1qTq1bgw4tNhJbsESfjA"
    USDC         = "TEkxiTehnzSmSe2XqrBj4w32RUN966rdz8"
    USDC_GemJoin = "TRGTuMiDYAbztetdndYyMzYvtaRmucjz5q"
    USDC_PSM     = "TUcj1rpMgJCcFZULyq7uLbkmfh9xMnYTmA"
    VAT          = "TBbYhvifBJVQ5ytThJ5ZfHfX8mK133ccqv"
)

type PSM struct {
    topic string

    // check values
    cBalanceOfUSDD  *big.Int
    cBalanceOfUSDT  *big.Int
    cBalanceOfUSDC  *big.Int
    isLowUSDDWarned bool

    // report values
    rBalanceOfUSDD *big.Int
    rBalanceOfUSDT *big.Int
    rBalanceOfUSDC *big.Int

    // stats values
    sBalanceOfUSDD *big.Int
    sBalanceOfUSDT *big.Int
    sBalanceOfUSDC *big.Int
    sTime          time.Time
}

func StartPSM(c *cron.Cron, concerned map[string]func(event *net.Event)) {
    psm := &PSM{topic: "PSM", sTime: time.Now()}
    psm.init()

    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" */10 * * * ?", misc.WrapLog(psm.check))
    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 0 */1 * * ?", misc.WrapLog(psm.report))
    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 30 */6 * * ?", misc.WrapLog(psm.stats))

    concerned[USDT_PSM] = psm.handleUSDT
    concerned[USDC_PSM] = psm.handleUSDC
}

func (p *PSM) handleUSDT(event *net.Event) {
    p.handleGemEvents(event, "USDT")
}

func (p *PSM) handleUSDC(event *net.Event) {
    p.handleGemEvents(event, "USDC")
}

func (p *PSM) handleGemEvents(event *net.Event, ilk string) {
    amount, _ := new(big.Int).SetString(event.Result["value"], 10)
    amount = misc.ConvertDec6(amount)
    if strings.Compare(event.EventName, "BuyGem") == 0 {
        amount = amount.Neg(amount)
    }
    if amount.CmpAbs(big.NewInt(config.Get().PSM.GemThreshold)) >= 0 {
        slack.SendMsg(p.topic, "Large %s, %s, %s, %s <!channel>",
            event.EventName,
            misc.FormatTokenAmt(ilk, amount, true),
            misc.FormatUser(net.GetTxFrom(event.TransactionHash)),
            misc.FormatTxUrl(event.TransactionHash))
    }
}

func (p *PSM) init() {
    p.cBalanceOfUSDD, p.cBalanceOfUSDT, p.cBalanceOfUSDC = p.getUSDDBalance(), p.getUSDTBalance(), p.getUSDCBalance()
    p.rBalanceOfUSDD, p.rBalanceOfUSDT, p.rBalanceOfUSDC = big.NewInt(-1), big.NewInt(-1), big.NewInt(-1)
    p.sBalanceOfUSDD, p.sBalanceOfUSDT, p.sBalanceOfUSDC = p.cBalanceOfUSDD, p.cBalanceOfUSDT, p.cBalanceOfUSDC
    p.report()
}

func (p *PSM) check() {
    // check if each ilk`s balance change big
    reportThreshold := big.NewInt(config.Get().PSM.ReportThreshold)
    balanceOfUSDT := p.getUSDTBalance()
    diffOfUSDT := big.NewInt(0)
    diffOfUSDT = diffOfUSDT.Sub(balanceOfUSDT, p.cBalanceOfUSDT)
    if diffOfUSDT.CmpAbs(reportThreshold) >= 0 {
        slack.SendMsg(p.topic, "Large gem balance change in last `10min`, %s <!channel>",
            misc.FormatTokenAmt("USDT", diffOfUSDT, true))
        p.report()
    }
    p.cBalanceOfUSDT = balanceOfUSDT

    balanceOfUSDC := p.getUSDCBalance()
    diffOfUSDC := big.NewInt(0)
    diffOfUSDC = diffOfUSDC.Sub(balanceOfUSDC, p.cBalanceOfUSDC)
    if diffOfUSDC.CmpAbs(reportThreshold) >= 0 {
        slack.SendMsg(p.topic, "Large gem balance change in last `10min`, %s <!channel>",
            misc.FormatTokenAmt("USDC", diffOfUSDC, true))
        p.report()
    }
    p.cBalanceOfUSDC = balanceOfUSDC

    // check if Vault remained USDD balance lower than threshold
    balanceOfUSDD := p.getUSDDBalance()
    daiThreshold := big.NewInt(config.Get().PSM.DaiThreshold)
    if !p.isLowUSDDWarned && balanceOfUSDD.CmpAbs(daiThreshold) < 0 {
        p.isLowUSDDWarned = true
        slack.SendMsg(p.topic, "Vault remained USDD balance lower than %s <!channel>",
            misc.ToReadableDec(daiThreshold))
    }
    if balanceOfUSDD.CmpAbs(daiThreshold) >= 0 {
        p.isLowUSDDWarned = false
    }
    p.cBalanceOfUSDD = balanceOfUSDD
}

func (p *PSM) report() {
    balanceOfUSDD, balanceOfUSDT, balanceOfUSDC := p.getUSDDBalance(), p.getUSDTBalance(), p.getUSDCBalance()
    slack.SendMsg(p.topic, "State Report, %s, %s, %s",
        misc.FormatTokenAmt("USDD", balanceOfUSDD, false),
        misc.FormatTokenAmt("USDT", balanceOfUSDT, false),
        misc.FormatTokenAmt("USDC", balanceOfUSDC, false))
    p.rBalanceOfUSDD, p.rBalanceOfUSDT, p.rBalanceOfUSDC = balanceOfUSDD, balanceOfUSDT, balanceOfUSDC
}

func (p *PSM) stats() {
    balanceOfUSDD, balanceOfUSDT, balanceOfUSDC, now := p.getUSDDBalance(), p.getUSDTBalance(), p.getUSDCBalance(), time.Now()
    slack.SendMsg(p.topic, "Stats Report, from `%s` ~ `%s`, %s, %s, %s",
        p.sTime.Format("15:04"), now.Format("15:04"),
        misc.FormatTokenAmt("USDD", p.sBalanceOfUSDD.Sub(balanceOfUSDD, p.sBalanceOfUSDD), true),
        misc.FormatTokenAmt("USDT", p.sBalanceOfUSDT.Sub(balanceOfUSDT, p.sBalanceOfUSDT), true),
        misc.FormatTokenAmt("USDC", p.sBalanceOfUSDC.Sub(balanceOfUSDC, p.sBalanceOfUSDC), true))
    p.sBalanceOfUSDD, p.sBalanceOfUSDT, p.sBalanceOfUSDC, p.sTime = balanceOfUSDD, balanceOfUSDT, balanceOfUSDC, now
}

func (p *PSM) getUSDDBalance() *big.Int {
    result, err := net.Trigger(USDD_DaiJoin, "getUsddBalance()", "")
    if err != nil {
        // if we cannot get current USDD balance, return the c-value
        misc.Warn(p.topic+".getUSDDBalance", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query USDD balance", err.Error()))
        return p.cBalanceOfUSDD
    }
    return misc.ConvertDec6(misc.ToBigInt(result))
}

func (p *PSM) getUSDTBalance() *big.Int {
    result, err := net.Trigger(USDT, "balanceOf(address)", misc.ToEthAddr(USDT_GemJoin))
    if err != nil {
        // if we cannot get current USDT balance, return the c-value
        misc.Warn(p.topic+".getUSDTBalance", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query USDT balance", err.Error()))
        return p.cBalanceOfUSDT
    }
    return misc.ConvertDec6(misc.ToBigInt(result))
}

func (p *PSM) getUSDCBalance() *big.Int {
    result, err := net.Trigger(USDC, "balanceOf(address)", misc.ToEthAddr(USDC_GemJoin))
    if err != nil {
        // if we cannot get current USDC balance, return the c-value
        misc.Warn(p.topic+".getUSDCBalance", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query USDC balance", err.Error()))
        return p.cBalanceOfUSDC
    }
    return misc.ConvertDec6(misc.ToBigInt(result))
}
