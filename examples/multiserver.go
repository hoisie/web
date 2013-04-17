package main

import (
	"github.com/xyproto/web"
)

func hello1(val string) string { return "hello1 " + val }

func hello2(val string) string { return "hello2 " + val }

func main() {
	var server1, server2 web.Server

	server1.Get("/(.*)", hello1)
	go server1.Run("0.0.0.0:9999")
	server2.Get("/(.*)", hello2)
	go server2.Run("0.0.0.0:8999")
	<-make(chan int)
}
