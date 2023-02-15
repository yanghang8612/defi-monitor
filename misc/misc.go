package misc

import (
	"psm-monitor/config"

	"fmt"
	"math/big"
	"reflect"
	"runtime"
	"strings"
	"time"
)

var tokenMapping = map[string]string{
	"usdt": ":usdtlogo:",
	"usdc": ":usdclogo:",
	"tusd": ":tusdlogo:",
	"usdj": ":usdjlogo:",
	"usdd": ":usdd:",
}

func GetTokenLogo(token string) string {
	return tokenMapping[strings.ToLower(token)]
}

func FormatTokenAmt(token string, amt *big.Int, isDiff bool) string {
	text := fmt.Sprintf("%s - `%s`", GetTokenLogo(token), ToReadableDec(big.NewInt(0).Abs(amt)))
	if isDiff {
		if amt.Sign() > 0 {
			text += " :arrow_heading_up:"
		} else if amt.Sign() < 0 {
			text += " :arrow_heading_down:"
		} else {
			text += " :repeat:"
		}
	}
	return text
}

func FormatUser(addr string) string {
	if !strings.HasPrefix(addr, "T") {
		addr = ToTronAddr(addr)
	}
	return fmt.Sprintf(":clown_face: - `%s`", addr)
}

func FormatTxUrl(txHash string) string {
	return fmt.Sprintf(":clippy:<https://tronscan.io/#/transaction/%s|TxHash>", txHash)
}

func WrapLog(f func()) func() {
	return func() {
		startAt := time.Now()
		f()
		costMilli := time.Now().Sub(startAt).Milliseconds()
		Info("Scheduled task report", fmt.Sprintf("task=[%s] cost=%dms", getFunctionName(f, '/'), costMilli))
	}
}

type logLevel struct {
	levelMap map[string]uint8
}

func (l *logLevel) get(s string) uint8 {
	return l.levelMap[strings.ToUpper(s)]
}

var levels = logLevel{levelMap: map[string]uint8{"DEBUG": 0, "INFO": 1, "WARN": 2, "ERROR": 3}}

func Debug(title, content string) {
	record("DEBUG", title, content)
}

func Info(title, content string) {
	record("INFO", title, content)
}

func Warn(title, content string) {
	record("WARN", title, content)
}

func Error(title, content string) {
	record("ERROR", title, content)
}

func record(level, title, content string) {
	if levels.get(level) >= levels.get(config.Get().LogLevel) {
		fmt.Printf("%-5s[%s] %-32s %s\n", level, time.Now().Format("01-02|15:04:05.000"), title, content)
	}
}

func getFunctionName(i interface{}, seps ...rune) string {
	// get function full name
	fn := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()

	// split its name by seps
	fields := strings.FieldsFunc(fn, func(sep rune) bool {
		for _, s := range seps {
			if sep == s {
				return true
			}
		}
		return false
	})

	// fmt.Println(fields)
	if size := len(fields); size > 0 {
		return fields[size-1]
	}
	return ""
}
