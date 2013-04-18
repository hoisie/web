// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
