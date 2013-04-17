package main

import (
	"github.com/xyproto/web"
)

type Message struct {
	Greeting string
	Response string
}

func hello(val string) (Message, error) {
	msg := Message{val, "Hello " + val}
	return msg, nil
}

func plain(val string) ([]byte, error) {
	return []byte("Plain " + val), nil
}

func main() {
	web.Get("/plain/(.*)", plain)
	web.Get("/(.*)", hello)
	web.Run("0.0.0.0:9999")
}
