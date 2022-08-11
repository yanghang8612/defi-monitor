package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "math/big"
    "net/http"
    "strconv"
    "time"

    "github.com/BurntSushi/toml"
    "github.com/btcsuite/btcutil/base58"
    "github.com/ethereum/go-ethereum/common"
    "github.com/robfig/cron"
    "github.com/status-im/keycard-go/hexutils"
)

const (
    URL      = "https://api.trongrid.io/wallet/triggerconstantcontract"
    USDT     = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
    USDTJoin = "TMn5WeW8a8KH9o8rBQux4RCgckD2SuMZmS"
    USDDJoin = "TMgSSHn8APyUVViqXxtveqFEB7mBBeGqNP"
)

type Config struct {
    SlackWebhook string
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
    initMonitor()

    c := cron.New()
    _ = c.AddFunc("*/9 * * * * ?", check)
    _ = c.AddFunc("1 */10 * * * ?", report)
    _ = c.AddFunc("2 0 */1 * * ?", stats)
    c.Start()

    defer c.Stop()
    select {}
}

func initMonitor() {
    sBalanceOfUSDT = getUSDTBalanceOf(USDTJoin)
    sBalanceOfUSDD = getUSDDBalance()
    preBalanceOfUSDT = sBalanceOfUSDT

    sendSlackMsg("APP", "Monitor now started")
    report()
    stats()
}

func stats() {
    balanceOfUSDT := getUSDTBalanceOf(USDTJoin)
    balanceOfUSDD := getUSDDBalance()
    now := time.Now()
    sendSlackMsg("PSM", "Stats from `%s` ~ `%s`, USDT - %s, USDD - %s",
        sTime.Format("15:04"), now.Format("15:04"),
        toReadableDec(sBalanceOfUSDT.Sub(balanceOfUSDT, sBalanceOfUSDT), true),
        toReadableDec(sBalanceOfUSDD.Sub(balanceOfUSDD, sBalanceOfUSDD), true))
    sBalanceOfUSDT = balanceOfUSDT
    sBalanceOfUSDD = balanceOfUSDD
    sTime = now

    fmt.Printf("[%s] %s\n", time.Now().Format("01-02 15:04:05"), "Stats task completed")
}

func check() {
    // check if large sell or bug occurred
    balanceOfUSDT := getUSDTBalanceOf(USDTJoin)
    diff := big.NewInt(0)
    diff = diff.Sub(balanceOfUSDT, preBalanceOfUSDT)
    if diff.CmpAbs(big.NewInt(10_000)) > 0 {
        if diff.Sign() > 0 {
            sendSlackMsg("PSM", "Large sellGem - %s <!channel>", toReadableDec(diff, true))
        } else {
            sendSlackMsg("PSM", "Large buyGem - %s <!channel>", toReadableDec(diff, true))
        }
        report()
    }
    preBalanceOfUSDT = balanceOfUSDT

    // check if Vault remained USDD balance lower than 1M
    balanceOfUSDD := getUSDDBalance()
    if !isLowUSDDWarned && balanceOfUSDD.CmpAbs(big.NewInt(5_000_000)) < 0 {
        isLowUSDDWarned = true
        sendSlackMsg("PSM", "Vault remained USDD balance lower than 5M <!channel>")
    }
    if balanceOfUSDD.CmpAbs(big.NewInt(1_000_000)) >= 0 {
        isLowUSDDWarned = false
    }

    fmt.Printf("[%s] %s\n", time.Now().Format("01-02 15:04:05"), "Check task completed")
}

func report() {
    balanceOfUSDT := getUSDTBalanceOf(USDTJoin)
    balanceOfUSDD := getUSDDBalance()
    sendSlackMsg("PSM", "USDT balance - %s, USDD balance - %s",
        toReadableDec(balanceOfUSDT, false),
        toReadableDec(balanceOfUSDD, false))
    fmt.Printf("[%s] %s\n", time.Now().Format("01-02 15:04:05"), "Report task completed")
}

func sendSlackMsg(topic, format string, a ...any) {
    data, _ := json.Marshal(&SlackMessage{
        Text: fmt.Sprintf("[%s] %s", topic, fmt.Sprintf(format, a...)),
    })
    fmt.Println(string(data))
    doPost(getConfig().SlackWebhook, data)
}

func getConfig() *Config {
    var config Config
    data, err := toml.DecodeFile("./config.toml", &config)
    if err != nil {
        fmt.Println(data, err)
    }
    return &config
}

func getUSDTBalanceOf(user string) *big.Int {
    return convertDec6(toBigInt(query(USDT, "balanceOf(address)", toEthAddr(user))))
}

func getUSDDBalance() *big.Int {
    return convertDec6(toBigInt(query(USDDJoin, "getUsddBalance()", "")))
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

func toReadableDec(n *big.Int, symbol bool) string {
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

    readableDec := string(buf[i+1:])
    if symbol {
        if n.Sign() > 0 {
            return ":arrow_up_small: `" + readableDec + "`"
        } else if n.Sign() < 0 {
            return ":arrow_down_small: `" + readableDec[1:] + "`"
        } else {
            return ":arrows_counterclockwise: `0`"
        }
    } else {
        return "`" + readableDec + "`"
    }
}
