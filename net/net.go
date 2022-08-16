package net

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/ethereum/go-ethereum/common/hexutil"
    "github.com/status-im/keycard-go/hexutils"
    "io"
    "math/big"
    "net/http"
    "strings"
)

const (
    Endpoint    = "https://api.trongrid.io/"
    TriggerPath = "wallet/triggerconstantcontract"
    EventsPath  = "v1/blocks/%d/events?limit=100"
)

var ErrNoReturn = errors.New("net: no return data")
var ErrQueryFailed = errors.New("net: query failed")

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
    rspData, err := Post(Endpoint+TriggerPath, Request{
        OwnerAddress:     "T9yD14Nj9j7xAB4dbGeiX9h8unkKHxuWwb",
        ContractAddress:  addr,
        FunctionSelector: selector,
        Parameter:        param,
        Visible:          true,
    })
    if err != nil {
        return "", err
    }
    var queryRsp QueryResponse
    _ = json.Unmarshal(rspData, &queryRsp)
    if !queryRsp.RpcResult.TriggerResult {
        return "", ErrQueryFailed
    }
    if len(queryRsp.Result) > 0 {
        return queryRsp.Result[0], nil
    }
    return "", ErrNoReturn
}

func Get(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err == nil && resp.StatusCode == 200 {
        defer resp.Body.Close()
        if body, err := io.ReadAll(resp.Body); err == nil {
            return body, nil
        }
        return nil, err
    }
    return nil, err
}

func Post(url string, d interface{}) ([]byte, error) {
    reqData, err := json.Marshal(d)
    if err != nil {
        return nil, err
    }
    rsp, err := http.Post(url, "application/json", bytes.NewBuffer(reqData))
    if err == nil && rsp.StatusCode == 200 {
        defer rsp.Body.Close()
        if body, err := io.ReadAll(rsp.Body); err == nil {
            return body, err
        }
        return nil, err
    }
    return nil, err
}
