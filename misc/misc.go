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
        costMilli := time.Now().Sub(startAt).Milliseconds()
        Log("Scheduled task report", fmt.Sprintf("task=[%s] cost=%dms", getFunctionName(f, '/'), costMilli))
    }
}

func Debug(title, content string) {
    record("DEBUG", title, content)
}

func Log(title, content string) {
    record("INFO", title, content)
}

func Warn(title, content string) {
    record("WARN", title, content)
}

func Error(title, content string) {
    record("ERROR", title, content)
}

func record(level, title, content string) {
    fmt.Printf("%-5s[%s] %-32s %s\n", level, time.Now().Format("01-02|15:04:05.000"), title, content)
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
