package main

import (
	"log"
	"os"

	"github.com/xyproto/web"
)

func hello(val string) string { return "hello " + val }

func main() {
	f, err := os.Create("server.log")
	if err != nil {
		println(err)
		return
	}
	logger := log.New(f, "", log.Ldate|log.Ltime)
	web.Get("/(.*)", hello)
	web.SetLogger(logger)
	web.Run("0.0.0.0:9999")
}
