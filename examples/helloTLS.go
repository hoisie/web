package main

import (
	"github.com/xyproto/web.go"
)

func hello(val string) string { return "hello " + val }

func main() {
    web.Get("/(.*)", hello)
    // You can create an example cert using generate_cert.go include in pkg crypto/tls
    web.RunTLS(":9999","/tmp/cert.pem","/tmp/key.pem")
}
