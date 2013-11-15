package main

import (
	"github.com/hoisie/web"
	"net/http"
	"strconv"
	"time"
)

func hello(ctx *web.Context, num string) {
	flusher, _ := ctx.ResponseWriter.(http.Flusher)
	flusher.Flush()
	n, _ := strconv.ParseInt(num, 10, 64)
	for i := int64(0); i < n; i++ {
		ctx.WriteString("<br>hello world</br>")
		flusher.Flush()
		time.Sleep(1e9)
	}
}

func main() {
	web.Get("/([0-9]+)", hello)
	web.Run("0.0.0.0:9999")
}
