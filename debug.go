package main

import (
	"log"
	"os"
)

var dbg bool

func setDbg(enable bool) {
	if enable && isTTY() {
		dbg = true
	} else {
		dbg = false
	}
}

func dbgLogf(format string, args ...interface{}) {
	if !dbg {
		return
	}
	log.Printf(format, args...)
}

func isTTY() bool {
	st, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
