# nfsstats

a Go (golang) parser for /proc/self/mountstats NFS statistics

## Installation

    go get -u github.com/selfishman/nfsstats

## Usage

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/selfishman/nfsstats"
)

func main() {
    f, err := os.Open("/proc/self/mountstats")
    if err != nil {
        fmt.Println(err)
        os.Exit(9)
    }
    defer f.Close()

    stats, err := nfsstats.Parse(f)
    jsonRaw, _ := json.Marshal(stats)
    fmt.Println(string(jsonRaw))
}
```

## License

MPL 2.0

## Author

Blaine Fleming (<blaine@selfishman.net>)

