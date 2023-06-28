package abi

import (
	"math/big"

	"github.com/holiman/uint256"
	"github.com/status-im/keycard-go/hexutils"
	"psm-monitor/misc"
	"psm-monitor/net"
)

func PadUint256(num uint64) string {
	return hexutils.BytesToHex(uint256.NewInt(num).PaddedBytes(32))
}

func PadAddress(addr string) string {
	return hexutils.BytesToHex(uint256.NewInt(0).SetBytes20(hexutils.HexToBytes(addr)).PaddedBytes(32))
}

func Coins(addr string, i uint64) string {
	result, err := net.Trigger(addr, "coins(uint256)", PadUint256(i))
	if err != nil {
		return ""
	}
	return misc.ToTronAddr(result[24:])
}

func Name(addr string) string {
	result, err := net.Trigger(addr, "symbol()", "")
	if err != nil {
		return ""
	}
	return string(hexutils.HexToBytes(result)[64:68])
}

func Decimals(addr string) uint8 {
	result, err := net.Trigger(addr, "decimals()", "")
	if err != nil {
		return 18
	}
	return uint8(misc.ToBigInt(result).Uint64())
}

func Balances(addr string, i int) (*big.Int, error) {
	result, err := net.Trigger(addr, "balances(uint256)", hexutils.BytesToHex(uint256.NewInt(uint64(i)).PaddedBytes(32)))
	if err != nil {
		return big.NewInt(0), err
	}
	return misc.ToBigInt(result), nil
}
