package misc

import (
    "math/big"
    "strconv"

    "github.com/status-im/keycard-go/hexutils"
)

func ToBigInt(hexData string) *big.Int {
    return big.NewInt(0).SetBytes(hexutils.HexToBytes(hexData))
}

func ConvertDec6(amt *big.Int) *big.Int {
    return amt.Div(amt, GetDec(6))
}

func ConvertDec18(amt *big.Int) *big.Int {
    return amt.Div(amt, GetDec(18))
}

func ToReadableDec(n *big.Int, symbol bool) string {
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

func GetDec(d uint) *big.Int {
    decFloat, _ := new(big.Float).SetString("1e" + strconv.Itoa(int(d)))
    decInt, _ := decFloat.Int(new(big.Int))
    return decInt
}
