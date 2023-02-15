package misc

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
	"github.com/status-im/keycard-go/hexutils"
)

func ToEthAddr(tronAddr string) string {
	ethAddr, _, _ := base58.CheckDecode(tronAddr)
	return hexutils.BytesToHex(common.BytesToAddress(ethAddr).Hash().Bytes())
}

func ToTronAddr(ethAddr string) string {
	return base58.CheckEncode(common.BytesToAddress(common.FromHex(ethAddr)).Bytes(), 0x41)
}
