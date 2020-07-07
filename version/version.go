package version

import (
	"fmt"
	"runtime"
)

var (
	GitCommit string
	Version   = "0.4.1"
	BuildDate = ""
	GoVersion = runtime.Version()
	OsArch    = fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
)
