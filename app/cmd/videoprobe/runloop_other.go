//go:build !darwin

package main

import "os"

func runWithLoop(probe func() int) {
	os.Exit(probe())
}
