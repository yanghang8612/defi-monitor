package monitor

import (
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"psm-monitor/config"
	"psm-monitor/misc"
	"psm-monitor/net"
	"psm-monitor/slack"

	"github.com/robfig/cron"
)

type ilk struct {
	token   string
	gemJoin string
	psm     string
	decimal uint
}

const (
	USDD         = "USDD"
	USDD_DaiJoin = "TMgSSHn8APyUVViqXxtveqFEB7mBBeGqNP"
	USDT         = "USDT"
	USDC         = "USDC"
	TUSD         = "TUSD"
	USDJ         = "USDJ"
)

var ilkList = [...]string{"USDT", "USDC", "TUSD", "USDJ"}
var ilks = map[string]*ilk{
	USDT: {
		token:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		gemJoin: "TMn5WeW8a8KH9o8rBQux4RCgckD2SuMZmS",
		psm:     "TM9gWuCdFGNMiT1qTq1bgw4tNhJbsESfjA",
		decimal: 6,
	},
	USDC: {
		token:   "TEkxiTehnzSmSe2XqrBj4w32RUN966rdz8",
		gemJoin: "TRGTuMiDYAbztetdndYyMzYvtaRmucjz5q",
		psm:     "TUcj1rpMgJCcFZULyq7uLbkmfh9xMnYTmA",
		decimal: 6,
	},
	TUSD: {
		token:   "TUpMhErZL2fhh4sVNULAbNKLokS4GjC1F4",
		gemJoin: "TPxcmB9dQC3LHswCNEc4rJs1HFGb8McYjT",
		psm:     "TY2op6AKcEkFhv8hxNJj3FBUfjManxYLSe",
		decimal: 18,
	},
	USDJ: {
		token:   "TMwFHYXLJaRUPeW6421aqXL4ZEzPRFGkGT",
		gemJoin: "TKAovR61zwp1t9Rg1UE4UY5mXt7QTJdDXg",
		psm:     "TVS3rVDUSd3ySeXV5moRH2J2t5B9reJfLR",
		decimal: 18,
	},
}

type PSM struct {
	topic string
	ilks  map[string]*ilk

	isLowUSDDWarned bool

	// check balances for all tracked token
	cBalance map[string]*big.Int

	// report balances for all tracked token
	rBalance map[string]*big.Int

	// stats balances for all tracked token
	sBalance map[string]*big.Int
	sTime    time.Time
}

func StartPSM(c *cron.Cron, concerned map[string]func(event *net.Event)) {
	psm := &PSM{
		topic:    ":usdd: [PSM]",
		cBalance: make(map[string]*big.Int),
		rBalance: make(map[string]*big.Int),
		sBalance: make(map[string]*big.Int),
		sTime:    time.Now(),
	}
	psm.init()

	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" */10 * * * ?", misc.WrapLog(psm.check))
	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 0 */1 * * ?", misc.WrapLog(psm.report))
	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 30 */6 * * ?", misc.WrapLog(psm.stats))

	for _, name := range ilkList {
		concerned[ilks[name].psm] = psm.handleGemEvents
	}
}

func (p *PSM) handleGemEvents(event *net.Event) {
	var matchedName string
	for _, name := range ilkList {
		if strings.Compare(event.Address, ilks[name].psm) == 0 {
			matchedName = name
		}
	}
	amount, _ := new(big.Int).SetString(event.Result["value"], 10)
	amount = misc.ConvertDecN(amount, ilks[matchedName].decimal)
	if strings.Compare(event.EventName, "BuyGem") == 0 {
		amount = amount.Neg(amount)
	}
	if amount.CmpAbs(big.NewInt(config.Get().PSM.GemThreshold)) >= 0 {
		slack.SendMsg(p.topic, "Large %s, %s, %s, %s <!channel>",
			event.EventName,
			misc.FormatTokenAmt(matchedName, amount, true),
			misc.FormatUser(net.GetTxFrom(event.TransactionHash)),
			misc.FormatTxUrl(event.TransactionHash))
	}
}

func (p *PSM) init() {
	p.cBalance[USDD] = p.getUSDDBalance()
	p.rBalance[USDD] = big.NewInt(-1)
	p.sBalance[USDD] = p.cBalance[USDD]
	for _, name := range ilkList {
		p.cBalance[name] = p.getTokenBalance(name)
		p.rBalance[name] = big.NewInt(-1)
		p.sBalance[name] = p.cBalance[name]
	}
	p.report()
}

func (p *PSM) check() {
	// check if each ilk`s balance change big
	reportThreshold := big.NewInt(config.Get().PSM.ReportThreshold)
	for _, name := range ilkList {
		balanceOfToken := p.getTokenBalance(name)
		diff := big.NewInt(0)
		diff = diff.Sub(balanceOfToken, p.cBalance[name])
		if diff.CmpAbs(reportThreshold) >= 0 {
			slack.SendMsg(p.topic, "Large gem balance change in last `10min`, %s <!channel>",
				misc.FormatTokenAmt(name, diff, true))
			p.report()
		}
		p.cBalance[name] = balanceOfToken
	}

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
	p.cBalance[USDD] = balanceOfUSDD
}

func (p *PSM) report() {
	ilkReportStr := ""
	for _, name := range ilkList {
		p.rBalance[name] = p.getTokenBalance(name)
		ilkReportStr += ", " + misc.FormatTokenAmt(name, p.rBalance[name], false)
	}
	slack.SendMsg(p.topic, "State Report, %s%s",
		misc.FormatTokenAmt(USDD, p.getUSDDBalance(), false), ilkReportStr)
}

func (p *PSM) stats() {
	balanceOfUSDD, now := p.getUSDDBalance(), time.Now()
	ilkStatsStr := ""
	for _, name := range ilkList {
		balanceOfToken := p.getTokenBalance(name)
		ilkStatsStr += ", " + misc.FormatTokenAmt(name, p.sBalance[name].Sub(balanceOfToken, p.sBalance[name]), true)
	}
	slack.SendMsg(p.topic, "Stats Report, from `%s` ~ `%s`, %s%s",
		p.sTime.Format("15:04"), now.Format("15:04"),
		misc.FormatTokenAmt(USDD, p.sBalance[USDD].Sub(balanceOfUSDD, p.sBalance[USDD]), true),
		ilkStatsStr)
	p.sBalance[USDD], p.sTime = balanceOfUSDD, now
}

func (p *PSM) getUSDDBalance() *big.Int {
	result, err := net.Trigger(USDD_DaiJoin, "getUsddBalance()", "")
	if err != nil {
		// if we cannot get current USDD balance, return the c-value
		misc.Warn(p.topic+".getUSDDBalance", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query USDD balance", err.Error()))
		return p.cBalance[USDD]
	}
	return misc.ConvertDec6(misc.ToBigInt(result))
}

func (p *PSM) getTokenBalance(name string) *big.Int {
	result, err := net.Trigger(ilks[name].token, "balanceOf(address)", misc.ToEthAddr(ilks[name].gemJoin))
	if err != nil {
		// if we cannot get current balance, return the c-value
		misc.Warn(fmt.Sprintf("%s.get%sBalance", p.topic, name),
			fmt.Sprintf("action=\"query %s balance\" reason=\"%s\"", name, err.Error()))
		return p.cBalance[name]
	}
	return misc.ConvertDecN(misc.ToBigInt(result), ilks[name].decimal)
}
