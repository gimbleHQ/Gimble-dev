package platform

import (
    "fmt"
    "runtime"
)

var supportedOS = map[string]struct{}{
    "linux":  {},
    "darwin": {},
}

func EnsureSupported() error {
    if _, ok := supportedOS[runtime.GOOS]; ok {
        return nil
    }

    return fmt.Errorf("gimble supports linux and macOS only (current: %s)", runtime.GOOS)
}
