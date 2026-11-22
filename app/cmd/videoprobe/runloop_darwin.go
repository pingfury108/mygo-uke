package main

import (
	"os"
	"runtime"

	"github.com/ebitengine/purego"
)

var runLoop func()

func init() {
	runtime.LockOSThread()
	cf, err := purego.Dlopen(
		"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation",
		purego.RTLD_GLOBAL|purego.RTLD_NOW,
	)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&runLoop, cf, "CFRunLoopRun")
}

func runWithLoop(probe func() int) {
	go func() { os.Exit(probe()) }()
	runLoop()
}
