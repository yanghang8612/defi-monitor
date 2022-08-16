package misc

import (
    "fmt"
    "reflect"
    "runtime"
    "strings"
    "time"
)

func FormatTxUrl(txHash string) string {
    return fmt.Sprintf(":clippy:<https://tronscan.org/#/transaction/%s|TxHash>", txHash)
}

func WrapLog(f func()) func() {
    return func() {
        startAt := time.Now()
        f()
        endAt := time.Now()
        fmt.Printf("[%s] %s task completed, cost %d ms\n", endAt.Format("01-02 15:04:05"), getFunctionName(f, '/'), endAt.Sub(startAt).Milliseconds())
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
