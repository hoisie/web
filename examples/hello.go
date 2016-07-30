package main

import (
	"github.com/hoisie/web"
)

func hello(val string) string { return "hello " + val + "\n" }

func main() {
	web.Get("/(.*)", hello)
	web.Run("0.0.0.0:9999")
}
