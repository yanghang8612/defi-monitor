package net

import (
    "psm-monitor/misc"

    "bytes"
    "crypto/tls"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "math/big"
    "math/rand"
    "net"
    "net/http"
    "strings"
    "time"

    "github.com/ethereum/go-ethereum/common/hexutil"
    "github.com/status-im/keycard-go/hexutils"
)

const (
    Endpoint    = "https://api.trongrid.io/"
    TriggerPath = "wallet/triggerconstantcontract"
    EventsPath  = "v1/blocks/%d/events?limit=100"
)

var ErrHttpFailed = errors.New("net: http request failed")
var ErrNoReturn = errors.New("net: no return data")
var ErrQueryFailed = errors.New("net: query failed")

var defaultTransport = &http.Transport{
    Proxy: http.ProxyFromEnvironment,
    DialContext: (&net.Dialer{
        Timeout:   30 * time.Second,
        KeepAlive: 30 * time.Second,
    }).DialContext,
    ForceAttemptHTTP2:     true,
    MaxIdleConns:          100,
    IdleConnTimeout:       90 * time.Second,
    TLSHandshakeTimeout:   10 * time.Second,
    ExpectContinueTimeout: 1 * time.Second,
    TLSClientConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
}

var defaultHTTPClient = &http.Client{
    Transport: defaultTransport,
    Timeout:   3 * time.Second,
}

type Request struct {
    OwnerAddress     string `json:"owner_address"`
    ContractAddress  string `json:"contract_address"`
    FunctionSelector string `json:"function_selector"`
    Parameter        string `json:"parameter"`
    Visible          bool   `json:"visible"`
}

type QueryResponse struct {
    Result    []string `json:"constant_result"`
    RpcResult struct {
        TriggerResult bool `json:"result"`
    } `json:"result"`
}

type JsonRpcMessage struct {
    Version string `json:"jsonrpc,omitempty"`
    ID      int64  `json:"id,omitempty"`
    Method  string `json:"method,omitempty"`
    Params  string `json:"params,omitempty"`
    Error   error  `json:"error,omitempty"`
    Result  string `json:"result,omitempty"`
}

func newJsonRpcMessage(method string, params []byte) *JsonRpcMessage {
    return &JsonRpcMessage{
        Version: "2.0",
        ID:      233,
        Method:  method,
        Params:  hexutils.BytesToHex(params),
    }
}

func CallJsonRpc(method string, params []byte) []byte {
    data, err := Post(Endpoint+"jsonrpc", newJsonRpcMessage(method, params))
    if err != nil {
        return nil
    }
    var rspMsg JsonRpcMessage
    if err := json.Unmarshal(data, &rspMsg); err == nil {
        if len(rspMsg.Result)%2 == 1 {
            return hexutil.MustDecode(strings.ReplaceAll(rspMsg.Result, "0x", "0x0"))
        }
        return hexutil.MustDecode(rspMsg.Result)
    }
    return nil
}

func BlockNumber() uint64 {
    return new(big.Int).SetBytes(CallJsonRpc("eth_blockNumber", nil)).Uint64()
}

func GetBlockEvents(blockNumber uint64) []Event {
    allEvents := make([]Event, 0)
    events := Events{}
    events.Meta.Links.Next = Endpoint + fmt.Sprintf(EventsPath, blockNumber)
    for len(events.Meta.Links.Next) != 0 {
        rspData, err := Get(events.Meta.Links.Next)
        if err != nil {
            break
        }
        events = Events{}
        if err := json.Unmarshal(rspData, &events); err == nil {
            allEvents = append(allEvents, events.Data...)
        }
    }
    return allEvents
}

func Query(addr, selector, param string) (string, error) {
    resData, err := Post(Endpoint+TriggerPath, Request{
        OwnerAddress:     "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb",
        ContractAddress:  addr,
        FunctionSelector: selector,
        Parameter:        param,
        Visible:          true,
    })
    if err != nil {
        return "", err
    }
    var queryRes QueryResponse
    _ = json.Unmarshal(resData, &queryRes)
    if !queryRes.RpcResult.TriggerResult {
        return "", ErrQueryFailed
    }
    if len(queryRes.Result) > 0 {
        return queryRes.Result[0], nil
    }
    return "", ErrNoReturn
}

func Get(url string) ([]byte, error) {
    req, _ := http.NewRequest("GET", url, nil)
    return doRequestWithRetry(req, []byte("nil"))
}

func Post(url string, d interface{}) ([]byte, error) {
    reqData, jsonErr := json.Marshal(d)
    if jsonErr != nil {
        return nil, jsonErr
    }
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(reqData))
    req.Header.Set("Content-Type", "application/json")
    return doRequestWithRetry(req, reqData)
}

func doRequestWithRetry(req *http.Request, body []byte) ([]byte, error) {
    reqId := rand.Uint32()
    misc.Log("Http request start", fmt.Sprintf("url=%s method=%-4s data=%s reqid=%d", req.URL, req.Method, string(body), reqId))
    for i := 1; i <= 3; i++ {
        startAt := time.Now()
        retRes, retErr := defaultHTTPClient.Do(req)
        cost := time.Now().Sub(startAt).Milliseconds()
        if retErr == nil && retRes.StatusCode == 200 {
            if body, ioErr := io.ReadAll(retRes.Body); ioErr == nil {
                _ = retRes.Body.Close()
                misc.Log("Http request success", fmt.Sprintf("reqid=%d cost=%dms", reqId, cost))
                return body, nil
            }
            _ = retRes.Body.Close()
        }
        if retErr != nil {
            misc.Warn("Http request retry", fmt.Sprintf("reqid=%d cost=%dms times=%dth reason=\"%s\"", reqId, cost, i, retErr.Error()))
        } else {
            misc.Warn("Http request retry", fmt.Sprintf("reqid=%d cost=%dms times=%dth reason=\"invalid status code %d\"", reqId, cost, i, retRes.StatusCode))
        }
    }
    misc.Error("Http request failed", fmt.Sprintf("reqid=%d reason=\"retry exceed three times\"", reqId))
    return nil, ErrHttpFailed
}
