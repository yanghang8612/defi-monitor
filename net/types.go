package net

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
    Success bool    `json:"success"`
    Data    []Event `json:"data"`
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
