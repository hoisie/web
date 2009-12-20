package main

import (
    "fmt"
    "web"
)

func hello(val string) string { return fmt.Sprintf("hello %s", val) }

func main() {
    web.Get("/(.*)", hello)
    web.Run("0.0.0.0:9999")
}
