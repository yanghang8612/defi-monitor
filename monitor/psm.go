package monitor

import (
    "psm-monitor/config"
    "psm-monitor/misc"
    "psm-monitor/net"
    "psm-monitor/slack"

    "math/big"
    "math/rand"
    "strconv"
    "time"

    "github.com/robfig/cron"
)

const (
    USDT     = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
    USDTJoin = "TMn5WeW8a8KH9o8rBQux4RCgckD2SuMZmS"
    USDDJoin = "TMgSSHn8APyUVViqXxtveqFEB7mBBeGqNP"
)

type PSM struct {
    topic string

    // check values
    cBalanceOfUSDD  *big.Int
    cBalanceOfUSDT  *big.Int
    isLowUSDDWarned bool

    // report values
    rBalanceOfUSDD *big.Int
    rBalanceOfUSDT *big.Int

    // stats values
    sBalanceOfUSDD *big.Int
    sBalanceOfUSDT *big.Int
    sTime          time.Time
}

func StartPSM(c *cron.Cron, _ map[string]func(event *net.Event)) {
    psm := &PSM{topic: "PSM", sTime: time.Now()}
    psm.init()

    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%9))+"/9 * * * * ?", misc.WrapLog(psm.check))
    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" */10 * * * ?", misc.WrapLog(psm.report))
    _ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 0 */1 * * ?", misc.WrapLog(psm.stats))
}

func (p *PSM) init() {
    p.cBalanceOfUSDD, p.cBalanceOfUSDT = p.getUSDDBalance(), p.getUSDTBalance()
    p.rBalanceOfUSDD, p.rBalanceOfUSDT = big.NewInt(-1), big.NewInt(-1)
    p.sBalanceOfUSDD, p.sBalanceOfUSDT = p.cBalanceOfUSDD, p.cBalanceOfUSDT
    p.report()
}

func (p *PSM) check() {
    // check if large sell or big happened
    balanceOfUSDT := p.getUSDTBalance()
    diff := big.NewInt(0)
    diff = diff.Sub(balanceOfUSDT, p.cBalanceOfUSDT)
    if diff.CmpAbs(big.NewInt(config.Get().PSM.GemThreshold)) > 0 {
        if diff.Sign() > 0 {
            slack.SendMsg(p.topic, "Large sellGem - %s <!channel>", misc.ToReadableDec(diff, true))
        } else {
            slack.SendMsg(p.topic, "Large buyGem - %s <!channel>", misc.ToReadableDec(diff, true))
        }
        p.report()
    }
    p.cBalanceOfUSDT = balanceOfUSDT

    // check if Vault remained USDD balance lower than threshold
    balanceOfUSDD := p.getUSDDBalance()
    threshold := big.NewInt(config.Get().PSM.DaiThreshold)
    if !p.isLowUSDDWarned && balanceOfUSDD.CmpAbs(threshold) < 0 {
        p.isLowUSDDWarned = true
        slack.SendMsg(p.topic, "Vault remained USDD balance lower than %s <!channel>",
            misc.ToReadableDec(threshold, false))
    }
    if balanceOfUSDD.CmpAbs(threshold) >= 0 {
        p.isLowUSDDWarned = false
    }
    p.cBalanceOfUSDD = balanceOfUSDD
}

func (p *PSM) report() {
    balanceOfUSDD, balanceOfUSDT := p.getUSDDBalance(), p.getUSDTBalance()
    if balanceOfUSDD.Cmp(p.rBalanceOfUSDD) != 0 || balanceOfUSDT.Cmp(p.rBalanceOfUSDT) != 0 {
        slack.SendMsg(p.topic, "USDD - %s, USDT - %s",
            misc.ToReadableDec(balanceOfUSDD, false),
            misc.ToReadableDec(balanceOfUSDT, false))
    }
    p.rBalanceOfUSDD, p.rBalanceOfUSDT = balanceOfUSDD, balanceOfUSDT
}

func (p *PSM) stats() {
    balanceOfUSDD, balanceOfUSDT, now := p.getUSDDBalance(), p.getUSDTBalance(), time.Now()
    slack.SendMsg(p.topic, "Stats from `%s` ~ `%s`, USDD - %s, USDT - %s",
        p.sTime.Format("15:04"), now.Format("15:04"),
        misc.ToReadableDec(p.sBalanceOfUSDD.Sub(balanceOfUSDD, p.sBalanceOfUSDD), true),
        misc.ToReadableDec(p.sBalanceOfUSDT.Sub(balanceOfUSDT, p.sBalanceOfUSDT), true))
    p.sBalanceOfUSDD, p.sBalanceOfUSDT, p.sTime = balanceOfUSDD, balanceOfUSDT, now
}

func (p *PSM) getUSDDBalance() *big.Int {
    result, err := net.Query(USDDJoin, "getUsddBalance()", "")
    if err != nil {
        // if we cannot get current USDD balance, return the pre value
        slack.ReportPanic(err.Error())
        return p.cBalanceOfUSDD
    }
    return misc.ConvertDec6(misc.ToBigInt(result))
}

func (p *PSM) getUSDTBalance() *big.Int {
    result, err := net.Query(USDT, "balanceOf(address)", misc.ToEthAddr(USDTJoin))
    if err != nil {
        // if we cannot get current USDT balance, return the c-value
        slack.ReportPanic(err.Error())
        return p.cBalanceOfUSDT
    }
    return misc.ConvertDec6(misc.ToBigInt(result))
}
