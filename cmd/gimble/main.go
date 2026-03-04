package main

import (
    "flag"
    "fmt"
    "os"
    "runtime"

    "github.com/gimble-dev/gimble/internal/platform"
)

var version = "dev"

func main() {
    showVersion := flag.Bool("version", false, "print gimble version")
    flag.Parse()

    if *showVersion {
        fmt.Printf("gimble %s\n", version)
        return
    }

    if err := platform.EnsureSupported(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }

    fmt.Printf("gimble is running on %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
