package main

import (
	"io"

	"github.com/hraban/web"
)

func root(ctx *web.Context, name string) error {
	ws := ctx.WebsockConn
	ws.Write([]byte("hey " + name))
	io.Copy(ws, ws)
	return nil
}

func main() {
	web.Websocket("/(.*)", root)
	web.Run("0.0.0.0:8081")
}
