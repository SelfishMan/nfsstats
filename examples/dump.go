package main

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/selfishman/nfsstats"
)

func main() {
    f, err := os.Open("mountstats.txt")
    if err != nil {
        fmt.Println(err)
        os.Exit(9)
    }
    defer f.Close()

    stats, err := nfsstats.Parse(f)
    jsonRaw, _ := json.Marshal(stats)
    fmt.Println(string(jsonRaw))
}

// vim:ft=go:et:ts=4:sw=4:sts=4:

