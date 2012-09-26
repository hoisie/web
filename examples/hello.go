package main

import (
   "github.com/jmrobles/web.go"
)

func hello(val string) string { return "hello " + val }

func main() {
    web.Get("/(.*)", hello)
    web.Run("0.0.0.0:9999")
}
