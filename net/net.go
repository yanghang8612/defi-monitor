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
    Endpoint         = "https://api.trongrid.io/"
    TriggerPath      = "wallet/triggerconstantcontract"
    BlockEventsPath  = "v1/blocks/%d/events?limit=200"
    LatestEventsPath = "v1/blocks/latest/events?limit=200"
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

func newJsonRpcMessage(method string, params []byte) *JsonRpcMessage {
    return &JsonRpcMessage{
        Version: "2.0",
        ID:      233,
        Method:  method,
        Params:  hexutils.BytesToHex(params),
    }
}

func CallJsonRpc(method string, params []byte) ([]byte, error) {
    data, err := Post(Endpoint+"jsonrpc", newJsonRpcMessage(method, params), nil)
    if err != nil {
        return nil, err
    }
    var rspMsg JsonRpcMessage
    if err := json.Unmarshal(data, &rspMsg); err == nil {
        if len(rspMsg.Result)%2 == 1 {
            return hexutil.MustDecode(strings.ReplaceAll(rspMsg.Result, "0x", "0x0")), nil
        }
        return hexutil.MustDecode(rspMsg.Result), nil
    } else {
        return nil, err
    }
}

func BlockNumber() uint64 {
    if resData, resErr := CallJsonRpc("eth_blockNumber", nil); resErr == nil {
        return new(big.Int).SetBytes(resData).Uint64()
    } else {
        return 0
    }
}

func GetBlockEvents(blockNumber uint64) []*Event {
    return getEvents(Endpoint + fmt.Sprintf(BlockEventsPath, blockNumber))
}

func GetLatestBlockEvents() []*Event {
    return getEvents(Endpoint + LatestEventsPath)
}

func getEvents(url string) []*Event {
    allEvents := make([]*Event, 0)
    events := Events{}
    events.Meta.Links.Next = url
    for len(events.Meta.Links.Next) != 0 {
        rspData, err := Get(events.Meta.Links.Next, nil)
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

func GetTxFrom(id string) string {
    if resData, netErr := Get("https://apilist.tronscanapi.com/api/transaction-info?hash="+id, nil); netErr == nil {
        result := make(map[string]interface{})
        if jsonErr := json.Unmarshal(resData, &result); jsonErr == nil {
            return result["ownerAddress"].(string)
        }
    }
    return ""
}

func Trigger(addr, selector, param string) (string, error) {
    resData, err := Post(Endpoint+TriggerPath, TriggerRequest{
        OwnerAddress:     "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb",
        ContractAddress:  addr,
        FunctionSelector: selector,
        Parameter:        param,
        Visible:          true,
    }, nil)
    if err != nil {
        return "", err
    }
    var queryRes TriggerResponse
    _ = json.Unmarshal(resData, &queryRes)
    if !queryRes.RpcResult.TriggerResult {
        return "", ErrQueryFailed
    }
    if len(queryRes.Result) > 0 {
        return queryRes.Result[0], nil
    }
    return "", ErrNoReturn
}

func Get(url string, chkFn func([]byte) error) ([]byte, error) {
    req, _ := http.NewRequest("GET", url, nil)
    return doRequestWithRetry(req, []byte("nil"), chkFn)
}

func Post(url string, d interface{}, chkFn func([]byte) error) ([]byte, error) {
    reqData, jsonErr := json.Marshal(d)
    if jsonErr != nil {
        return nil, jsonErr
    }
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(reqData))
    req.Header.Set("Content-Type", "application/json")
    return doRequestWithRetry(req, reqData, chkFn)
}

func doRequestWithRetry(req *http.Request, body []byte, chkFn func([]byte) error) ([]byte, error) {
    reqId := rand.Uint32()
    title := "Http request report"
    misc.Info(title, fmt.Sprintf("url=%s method=%s data=%s reqid=%d", req.URL, req.Method, string(body), reqId))
    for i := 1; i <= 3; i++ {
        startAt := time.Now()
        retRes, retErr := defaultHTTPClient.Do(req)
        cost := time.Now().Sub(startAt).Milliseconds()
        var chkErr error
        if retErr == nil && retRes.StatusCode == 200 {
            if body, ioErr := io.ReadAll(retRes.Body); ioErr == nil {
                _ = retRes.Body.Close()
                if chkFn != nil {
                    chkErr = chkFn(body)
                }
                if chkErr == nil {
                    misc.Debug(title, fmt.Sprintf("status=success reqid=%d cost=%dms", reqId, cost))
                    return body, nil
                }
            }
            _ = retRes.Body.Close()
        }
        if retErr != nil {
            misc.Debug(title, fmt.Sprintf("status=retry reqid=%d cost=%dms times=%dth reason=\"%s\"", reqId, cost, i, retErr.Error()))
        } else if chkErr != nil {
            misc.Debug(title, fmt.Sprintf("status=retry reqid=%d cost=%dms times=%dth reason=\"%s\"", reqId, cost, i, chkErr.Error()))
        } else {
            misc.Debug(title, fmt.Sprintf("status=retry reqid=%d cost=%dms times=%dth reason=\"invalid status code %d\"", reqId, cost, i, retRes.StatusCode))
        }
    }
    misc.Error(title, fmt.Sprintf("status=failed reqid=%d reason=\"retry exceed three times\"", reqId))
    return nil, ErrHttpFailed
}
