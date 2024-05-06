package main

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/depot/depot-go/logger"
)

func main() {
	logger.SetLogger(slog.Default())
	logger.With("key", "value").Info("Hello, world!")

	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		fmt.Printf("%+v\n", buildInfo)
		fmt.Printf("Main: %+v\n", buildInfo.Main)

		for _, dep := range buildInfo.Deps {
			fmt.Printf("Dep: %+v\n", dep)
		}

		for _, s := range buildInfo.Settings {
			fmt.Printf("Setting: %+v\n", s)
		}
	}
}
