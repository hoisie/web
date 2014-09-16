// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"

	"github.com/xyproto/web"
)

// Custom logging

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
	web.Run(":3000")
}
