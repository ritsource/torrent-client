package output

import (
	"fmt"
)

// DevMode v .
var DevMode bool

// v .
const (
	OutColRed    = "\033[0;31m"
	OutColGreen  = "\033[0;32m"
	OutColYellow = "\033[0;33m"
	OutColBlue   = "\033[0;34m"
	OutColNone   = "\033[0m"
)

// DevInfof .
func DevInfof(s string, v ...interface{}) {
	if DevMode {
		fmt.Printf("%v[INFO] %v", OutColGreen, OutColNone)
		fmt.Printf(s, v...)
	}
}

// DevErrorf .
func DevErrorf(s string, v ...interface{}) {
	if DevMode {
		fmt.Printf("%v[ERRO] %v", OutColRed, OutColNone)
		fmt.Printf(s, v...)
	}
}

// DevWarnf .
func DevWarnf(s string, v ...interface{}) {
	if DevMode {
		fmt.Printf("%v[WARN] %v", OutColYellow, OutColNone)
		fmt.Printf(s, v...)
	}
}

// Printf .
func Printf(s string, v ...interface{}) {
	fmt.Printf(s, v...)
}
