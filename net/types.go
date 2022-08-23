package net

type TriggerRequest struct {
    OwnerAddress     string `json:"owner_address"`
    ContractAddress  string `json:"contract_address"`
    FunctionSelector string `json:"function_selector"`
    Parameter        string `json:"parameter"`
    Visible          bool   `json:"visible"`
}

type TriggerResponse struct {
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

type Event struct {
    BlockNumber     uint64            `json:"block_number"`
    BlockTimestamp  int64             `json:"block_timestamp"`
    Address         string            `json:"contract_address"`
    LogIndex        uint              `json:"event_index"`
    EventName       string            `json:"event_name"`
    Event           string            `json:"event"`
    TransactionHash string            `json:"transaction_id"`
    Result          map[string]string `json:"result"`
}

type Events struct {
    Success bool     `json:"success"`
    Data    []*Event `json:"data"`
    Meta    struct {
        At    uint64 `json:"at"`
        Links struct {
            At   uint64 `json:"at"`
            Next string `json:"next"`
        } `json:"links"`
    }
}

type Block struct {
}
