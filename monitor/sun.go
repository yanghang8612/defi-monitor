package monitor

import (
	"psm-monitor/abi"
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

	"github.com/robfig/cron"
)

const (
	USDD_2Pool_Name = "USDD-2pool"
	USDD_2Pool      = "TNTfaTpkdd4AQDeqr8SGG7tgdkdjdhbP5c"
	TUSD_2Pool_Name = "TUSD-2pool"
	TUSD_2Pool      = "TS8d3ZrSxiGZkqhJqMzFKHEC1pjaowFMBJ"
)

type pool struct {
	name string
	addr string

	coinsAddr []string
	coinsName []string
	coinsDec  []uint8

	// check balances for this pool
	cPoolBalances []*big.Int

	// report balances for this pool
	rPoolBalances []*big.Int
	preA          int64

	// stats balances for this pool
	sPoolBalances []*big.Int
}

func (p *pool) init(n int) {
	p.coinsAddr = make([]string, n)
	p.coinsName = make([]string, n)
	p.coinsDec = make([]uint8, n)
	p.cPoolBalances = make([]*big.Int, n)
	p.rPoolBalances = make([]*big.Int, n)
	p.sPoolBalances = make([]*big.Int, n)

	for i := 0; i < n; i++ {
		p.coinsAddr[i] = abi.Coins(p.addr, uint64(i))
		p.coinsName[i] = abi.Name(p.coinsAddr[i])
		p.coinsDec[i] = abi.Decimals(p.coinsAddr[i])

		p.cPoolBalances[i] = p.getPoolBalance(i)
		p.rPoolBalances[i] = big.NewInt(-1)
		p.sPoolBalances[i] = p.cPoolBalances[i]
	}
}

func (p *pool) getA() int64 {
	if result, err := net.Trigger(p.addr, "A()", ""); err == nil {
		return misc.ToBigInt(result).Int64()
	} else {
		// if we cannot get current pool A value, return the pre-value
		misc.Warn(p.name+".getA", fmt.Sprintf("action=\"%s\" reason=\"%s\"", "query A value", err.Error()))
		return p.preA
	}
}

func (p *pool) getPoolBalance(i int) *big.Int {
	if res, err := abi.Balances(p.addr, i); err == nil {
		return misc.ConvertDecN(res, p.coinsDec[i])
	} else {
		// if we cannot get current coin pool balance, return the c-value
		misc.Warn(p.name+".getPoolBalance", fmt.Sprintf("action=query \"%s\" pool balance in \"%s\" failed, reason=\"%s\"", p.coinsName[i], p.name, err.Error()))
		return p.cPoolBalances[i]
	}
}

type SUN struct {
	topic string

	// all tracked pools
	pools map[string]*pool
	sTime time.Time
}

type oneCoinTx struct {
	TriggerInfo struct {
		Parameter map[string]string
	} `json:"trigger_info"`
}

func StartSUN(c *cron.Cron, concerned map[string]func(event *net.Event)) {
	sun := &SUN{topic: ":sunio: [SUN]", sTime: time.Now()}

	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" */10 * * * ?", misc.WrapLog(sun.check))
	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 0 */1 * * ?", misc.WrapLog(sun.report))
	_ = c.AddFunc(strconv.Itoa(int(rand.Uint32()%60))+" 30 */6 * * ?", misc.WrapLog(sun.stats))

	sun.pools = make(map[string]*pool)
	sun.pools[USDD_2Pool_Name] = &pool{
		name: USDD_2Pool_Name,
		addr: USDD_2Pool,
	}
	sun.pools[USDD_2Pool_Name].init(2)
	sun.pools[TUSD_2Pool_Name] = &pool{
		name: TUSD_2Pool_Name,
		addr: TUSD_2Pool,
	}
	sun.pools[TUSD_2Pool_Name].init(2)

	for k, v := range sun.pools {
		concerned[k] = func(event *net.Event) {
			sun.handleSwapSwapPoolEvent(event, v.coinsName[0], v.coinsName[1], v.name)
		}
	}

	sun.init()
}

func (s *SUN) handleSwapSwapPoolEvent(event *net.Event, coin0, coin1, pool string) {
	switch event.EventName {
	case "TokenExchange":
		var (
			boughtToken string
			soldToken   string
		)
		boughtAmount, _ := new(big.Int).SetString(event.Result["tokens_bought"], 10)
		soldAmount, _ := new(big.Int).SetString(event.Result["tokens_sold"], 10)
		if strings.Compare(event.Result["sold_id"], "0") == 0 {
			// swap coin0 => coin1
			boughtToken = coin1
			boughtAmount = misc.ConvertDec6(boughtAmount)
			soldToken = coin0
			soldAmount = misc.ConvertDec18(soldAmount)
		} else {
			// swap coin1 => coin0
			boughtToken = coin0
			boughtAmount = misc.ConvertDec18(boughtAmount)
			soldToken = coin1
			soldAmount = misc.ConvertDec6(soldAmount)
		}
		diff := big.NewInt(0)
		diff = diff.Sub(soldAmount, boughtAmount)
		threshold := big.NewInt(config.Get().SUN.SwapThreshold)
		if boughtAmount.Cmp(threshold) > 0 {
			msg := appendWarningIfNeeded(fmt.Sprintf("Large %s, %s => %s, %s, ",
				event.EventName,
				misc.FormatTokenAmt(soldToken, soldAmount, false),
				misc.FormatTokenAmt(boughtToken, boughtAmount, false),
				misc.FormatUser(net.GetTxFrom(event.TransactionHash))), boughtToken)
			if diff.Sign() > 0 {
				msg += fmt.Sprintf("lose %s, slip - `%.3f%%`, ",
					misc.FormatTokenAmt(boughtToken, diff, false),
					float64(diff.Uint64())/float64(soldAmount.Uint64())*100)
			} else if diff.Sign() < 0 {
				msg += fmt.Sprintf("earn %s, slip - `%.3f%%`, ",
					misc.FormatTokenAmt(boughtToken, diff.Abs(diff), false),
					float64(diff.Uint64())/float64(soldAmount.Uint64())*100)
			}
			msg += misc.FormatTxUrl(event.TransactionHash)
			slack.SendMsg(s.topic, msg+" in `"+pool+"`")
		}
	case "AddLiquidity":
		s.reportLiquidityOperation(event, coin0, coin1, pool, false)
	case "RemoveLiquidity", "RemoveLiquidityImbalance":
		s.reportLiquidityOperation(event, coin0, coin1, pool, true)
	case "RemoveLiquidityOne":
		tokenAmount, _ := new(big.Int).SetString(event.Result["coin_amount"], 10)
		threshold := big.NewInt(config.Get().SUN.LiquidityThreshold)
		tokenName := ""
		rspData, _ := net.Get("https://apilist.tronscan.org/api/transaction-info?hash="+event.TransactionHash, nil)
		var tx oneCoinTx
		if err := json.Unmarshal(rspData, &tx); err == nil {
			if tx.TriggerInfo.Parameter["i"] == "0" {
				// remove coin0
				tokenAmount = misc.ConvertDec18(tokenAmount)
				tokenName = coin0
			} else {
				// remove coin1
				tokenAmount = misc.ConvertDec6(tokenAmount)
				tokenName = coin1
			}
			if tokenAmount.Cmp(threshold) >= 0 {
				msg := appendWarningIfNeeded(fmt.Sprintf("Large %s, %s, %s, %s",
					event.EventName,
					misc.FormatTokenAmt(tokenName, tokenAmount.Neg(tokenAmount), true),
					misc.FormatUser(net.GetTxFrom(event.TransactionHash)),
					misc.FormatTxUrl(event.TransactionHash)), tokenName)
				slack.SendMsg(s.topic, msg+" - ["+pool+"]")
			}
		}
	case "RampA":
		oldA, _ := new(big.Int).SetString(event.Result["old_A"], 10)
		newA, _ := new(big.Int).SetString(event.Result["new_A"], 10)
		slack.SendMsg(s.topic, "["+pool+"] Ramp A from  `%d` => `%d`, %s in `%s`",
			oldA, newA, misc.FormatTxUrl(event.TransactionHash), pool)
	}
}

func (s *SUN) reportLiquidityOperation(event *net.Event, coin0, coin1, pool string, isRemove bool) {
	tokenAmounts := strings.Split(event.Result["token_amounts"], "\n")
	changedLiquidityOfCoin0, _ := new(big.Int).SetString(tokenAmounts[0], 10)
	changedLiquidityOfCoin0 = misc.ConvertDec18(changedLiquidityOfCoin0)
	if isRemove {
		changedLiquidityOfCoin0 = changedLiquidityOfCoin0.Neg(changedLiquidityOfCoin0)
	}
	changedLiquidityOfCoin1, _ := new(big.Int).SetString(tokenAmounts[1], 10)
	changedLiquidityOfCoin1 = misc.ConvertDec6(changedLiquidityOfCoin1)
	if isRemove {
		changedLiquidityOfCoin1 = changedLiquidityOfCoin1.Neg(changedLiquidityOfCoin1)
	}
	threshold := big.NewInt(config.Get().SUN.LiquidityThreshold)
	if changedLiquidityOfCoin0.CmpAbs(threshold) >= 0 || changedLiquidityOfCoin1.CmpAbs(threshold) >= 0 {
		msg := fmt.Sprintf("Large %s, %s, %s, %s, %s",
			event.EventName,
			misc.FormatTokenAmt(coin0, changedLiquidityOfCoin0, true),
			misc.FormatTokenAmt(coin1, changedLiquidityOfCoin1, true),
			misc.FormatUser(net.GetTxFrom(event.TransactionHash)),
			misc.FormatTxUrl(event.TransactionHash))
		if changedLiquidityOfCoin0.Cmp(big.NewInt(0)) < 0 && strings.Compare(coin0, "USDT") == 0 || changedLiquidityOfCoin1.Cmp(big.NewInt(0)) < 0 && strings.Compare(coin1, "USDT") == 0 {
			msg = appendWarningIfNeeded(msg, "USDT")
		}
		slack.SendMsg(s.topic, msg+" in `"+pool+"`")
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
	s.report()
}

func (s *SUN) check() {
	for _, v := range s.pools {
		coin0PoolBalance, coin1PoolBalance := v.getPoolBalance(0), v.getPoolBalance(1)
		diffCoin0 := big.NewInt(0)
		diffCoin0 = diffCoin0.Sub(coin0PoolBalance, v.cPoolBalances[0])
		diffCoin1 := big.NewInt(0)
		diffCoin1 = diffCoin1.Sub(coin1PoolBalance, v.cPoolBalances[1])
		reportThreshold := big.NewInt(config.Get().SUN.ReportThreshold)
		if diffCoin0.CmpAbs(reportThreshold) >= 0 || diffCoin1.CmpAbs(reportThreshold) >= 0 {
			slack.SendMsg(s.topic, "Large pool balance change in last `10min`, %s, %s in `%s`",
				misc.FormatTokenAmt(v.coinsName[0], diffCoin0, true),
				misc.FormatTokenAmt(v.coinsName[1], diffCoin1, true),
				v.name)
		}
		v.cPoolBalances[0], v.cPoolBalances[1] = coin0PoolBalance, coin1PoolBalance
	}
}

func (s *SUN) report() {
	for _, v := range s.pools {
		coin0PoolBalance, coin1PoolBalance, curA := v.getPoolBalance(0), v.getPoolBalance(1), v.getA()
		coin0Float64 := float64(coin0PoolBalance.Uint64())
		coin1Float64 := float64(coin1PoolBalance.Uint64())
		totalFloat64 := coin0Float64 + coin1Float64
		var (
			coin0Ratio float64
			coin1Ratio float64
			format     string
		)
		if coin0PoolBalance.Cmp(coin1PoolBalance) > 0 {
			coin0Ratio = coin0Float64 / coin1Float64
			coin1Ratio = 1.0
			format = "`%.3f%%` : `%.3f%%` :curly_loop: `%.3f` : `%.0f`"
		} else {
			coin0Ratio = 1.0
			coin1Ratio = coin1Float64 / coin0Float64
			format = "`%.3f%%` : `%.3f%%` :curly_loop: `%.0f` : `%.3f`"
		}
		slack.SendMsg(s.topic, "["+v.name+"] State Report, %s, %s, A - `%d`, Ratio - "+format+" in `%s`",
			misc.FormatTokenAmt(v.coinsName[0], coin0PoolBalance, false),
			misc.FormatTokenAmt(v.coinsName[1], coin1PoolBalance, false),
			curA,
			coin0Float64*100/totalFloat64,
			coin1Float64*100/totalFloat64,
			coin0Ratio,
			coin1Ratio,
			v.name)
		v.rPoolBalances[0], v.rPoolBalances[0], v.preA = coin0PoolBalance, coin1PoolBalance, curA
	}
}

func (s *SUN) stats() {
	for _, v := range s.pools {
		coin0PoolBalance, coin1PoolBalance, now := v.getPoolBalance(0), v.getPoolBalance(1), time.Now()
		slack.SendMsg(s.topic, "Stats Report, from `%s` ~ `%s`, %s, %s in `%s`",
			s.sTime.Format("15:04"), now.Format("15:04"),
			misc.FormatTokenAmt(v.coinsName[0], v.sPoolBalances[0].Sub(coin0PoolBalance, v.sPoolBalances[0]), true),
			misc.FormatTokenAmt(v.coinsName[1], v.sPoolBalances[1].Sub(coin1PoolBalance, v.sPoolBalances[1]), true),
			v.name)
		v.sPoolBalances[0], v.sPoolBalances[1], s.sTime = coin0PoolBalance, coin1PoolBalance, now
	}
}
