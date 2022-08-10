package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/status-im/keycard-go/hexutils"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
	"github.com/robfig/cron"
)

const (
	URL      = "https://api.trongrid.io/wallet/triggerconstantcontract"
	USDT     = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	USDTJoin = "TMn5WeW8a8KH9o8rBQux4RCgckD2SuMZmS"
	USDDJoin = "TMgSSHn8APyUVViqXxtveqFEB7mBBeGqNP"
)

type AssetV2 struct {
	Key   string
	Value uint64
}

type Account struct {
	Assets []AssetV2 `json:"assetV2"`
}

type Query struct {
	OwnerAddress     string `json:"owner_address"`
	ContractAddress  string `json:"contract_address"`
	FunctionSelector string `json:"function_selector"`
	Parameter        string `json:"parameter"`
	Visible          bool   `json:"visible"`
}

type Response struct {
	Result    []string `json:"constant_result"`
	RpcResult struct {
		TriggerResult bool `json:"result"`
	} `json:"result"`
}

type SlackMessage struct {
	Text string `json:"text"`
}

var (
	preBalanceOfUSDT = big.NewInt(0)
	isLowUSDDWarned  = false

	// stats params
	sBalanceOfUSDT = big.NewInt(0)
	sBalanceOfUSDD = big.NewInt(0)
	sTime          = time.Now()
)

func main() {
	sendSlackMsg("[PSM Log]: Monitor now started\n")
	report()
	c := cron.New()
	_ = c.AddFunc("*/9 * * * * ?", check)
	_ = c.AddFunc("0 */3 * * * ?", report)
	_ = c.AddFunc("0 0 */1 * * ?", stats)
	c.Start()

	defer c.Stop()
	select {}
}

func stats() {
	balanceOfUSDT := getUSDTBalanceOf(USDTJoin)
	balanceOfUSDD := getUSDDBalanceOf(USDDJoin)
	diffUSDT := balanceOfUSDT.Sub(balanceOfUSDT, sBalanceOfUSDT)
	diffUSDD := balanceOfUSDD.Sub(balanceOfUSDD, sBalanceOfUSDD)
	now := time.Now()
	sendSlackMsg(fmt.Sprintf("[PSM Log]: Stats from `%s` ~ `%s`, USDT change - `%s`, USDD change - `%s`\n",
		sTime.Format("01-02 15:04:05"), now.Format("01-02 15:04:05"), diffUSDT, diffUSDD))
	sBalanceOfUSDT = balanceOfUSDT
	sBalanceOfUSDD = balanceOfUSDD
	sTime = now
}

func check() {
	// check if large sell or bug occurred
	balanceOfUSDT := getUSDTBalanceOf(USDTJoin)
	diff := big.NewInt(0)
	diff = diff.Sub(balanceOfUSDT, preBalanceOfUSDT)
	if preBalanceOfUSDT.Cmp(big.NewInt(0)) != 0 && diff.CmpAbs(big.NewInt(100_000)) > 0 {
		if diff.Sign() > 0 {
			sendSlackMsg(fmt.Sprintf("[PSM Log]: Large sellGem occurred - `%s` <!channel>\n", convertDec6(diff)))
		} else {
			sendSlackMsg(fmt.Sprintf("[PSM Log]: Large buyGem occurred - `%s` <!channel>\n", convertDec6(diff)))
		}
		report()
	}
	preBalanceOfUSDT = balanceOfUSDT

	// check if Vault remained USDD balance lower than 1M
	balanceOfUSDD := getUSDDBalanceOf(USDDJoin)
	if !isLowUSDDWarned && balanceOfUSDD.CmpAbs(big.NewInt(1_000_000)) < 0 {
		isLowUSDDWarned = true
		sendSlackMsg(fmt.Sprintf("[PSM Log]: Vault remained USDD balance lower than 1M <!channel>\n"))
	}
	if balanceOfUSDD.CmpAbs(big.NewInt(1_000_000)) >= 0 {
		isLowUSDDWarned = false
	}

	fmt.Printf("[%s] %s\n", time.Now().Format("01-02 15:04:05"), "Check task completed")
}

func report() {
	balanceOfUSDT := getUSDTBalanceOf(USDTJoin)
	balanceOfUSDD := getUSDDBalanceOf(USDDJoin)
	//fmt.Printf("[PSM Log]: USDT balance - `%s`, USDD balance - `%s`\n", toReadableDec(balanceOfUSDT), toReadableDec(balanceOfUSDD))
	sendSlackMsg(fmt.Sprintf("[PSM Log]: USDT balance - `%s`, USDD balance - `%s`\n", toReadableDec(balanceOfUSDT), toReadableDec(balanceOfUSDD)))
	fmt.Printf("[%s] %s\n", time.Now().Format("01-02 15:04:05"), "Report task completed")
}

func sendSlackMsg(msg string) {
	data, _ := json.Marshal(&SlackMessage{
		Text: msg,
	})
	doPost("https://hooks.slack.com/services/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", data)
}

func getUSDTBalanceOf(user string) *big.Int {
	return convertDec6(toBigInt(query(USDT, "balanceOf(address)", toEthAddr(user))))
}

func getUSDDBalanceOf(user string) *big.Int {
	data := doGet("https://api.trongrid.io/wallet/getaccount?address=" + user + "&visible=true")
	var account Account
	if err := json.Unmarshal(data, &account); err == nil {
		for _, asset := range account.Assets {
			if strings.Compare(asset.Key, "1004777") == 0 {
				return convertDec6(big.NewInt(int64(asset.Value)))
			}
		}
	}
	return big.NewInt(0)
}

func query(addr, selector, param string) string {
	reqData, _ := json.Marshal(&Query{
		OwnerAddress:     "TGArstQjuME6fjBmEXVMdkGZNufxEDT6QB",
		ContractAddress:  addr,
		FunctionSelector: selector,
		Parameter:        param,
		Visible:          true,
	})
	rspData := doPost(URL, reqData)
	var result Response
	_ = json.Unmarshal(rspData, &result)
	if !result.RpcResult.TriggerResult {
		fmt.Println(addr + " trigger failed.")
	}
	if len(result.Result) > 0 {
		return result.Result[0]
	}
	return "no return"
}

func doGet(url string) []byte {
	resp, err := http.Get(url)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		if body, err := io.ReadAll(resp.Body); err == nil {
			return body
		}
	}
	return nil
}

func doPost(url string, data []byte) []byte {
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		if body, err := io.ReadAll(resp.Body); err == nil {
			return body
		}
	}
	return nil
}

func toBigInt(hexData string) *big.Int {
	return big.NewInt(0).SetBytes(hexutils.HexToBytes(hexData))
}

func toEthAddr(tronAddress string) string {
	ethAddr, _, _ := base58.CheckDecode(tronAddress)
	return hexutils.BytesToHex(common.BytesToAddress(ethAddr).Hash().Bytes())
}

func convertDec6(amt *big.Int) *big.Int {
	return amt.Div(amt, getDec(6))
}

func getDec(d uint) *big.Int {
	decFloat, _ := new(big.Float).SetString("1e" + strconv.Itoa(int(d)))
	decInt, _ := decFloat.Int(new(big.Int))
	return decInt
}

func toReadableDec(n *big.Int) string {
	var (
		text  = n.String()
		buf   = make([]byte, len(text)+len(text)/3)
		comma = 0
		i     = len(buf) - 1
	)
	for j := len(text) - 1; j >= 0; j, i = j-1, i-1 {
		c := text[j]

		switch {
		case c == '-':
			buf[i] = c
		case comma == 3:
			buf[i] = ','
			i--
			comma = 0
			fallthrough
		default:
			buf[i] = c
			comma++
		}
	}
	return string(buf[i+1:])
}
