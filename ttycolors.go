package web

import (
	"golang.org/x/crypto/ssh/terminal"
	"syscall"
)

var ttyCodes struct {
	green string
	white string
	reset string
}

func init() {
	ttyCodes.green = ttyBold("32")
	ttyCodes.white = ttyBold("37")
	ttyCodes.reset = ttyEscape("0")
}

func ttyBold(code string) string {
	return ttyEscape("1;" + code)
}

func ttyEscape(code string) string {
	if terminal.IsTerminal(syscall.Stdout) {
		return "\x1b[" + code + "m"
	} else {
		return ""
	}
}
