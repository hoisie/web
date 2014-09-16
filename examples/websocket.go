// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"io"

	"github.com/xyproto/web"
)

// Using websockets

func root(ctx *web.Context, name string) error {
	ws := ctx.WebsockConn
	ws.Write([]byte("hey " + name))
	_, err := io.Copy(ws, ws)
	return err
}

func main() {
	web.Websocket("/(.*)", root)
	web.Run(":3000")
}
