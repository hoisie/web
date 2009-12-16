# web.go

web.go is the simplest way to write web applications in the Go programming language. 

## Installation

1. Make sure you have Go installed and your environment is set up correctly: $GOROOT, $GOARCH, $GOBIN, etc.
2. Checkout the code
3. cd web.go && make install

## Example

    package main

    import (
      "web";
    )

    var urls = map[string] interface {} {
      "/(.*)" : hello,
    }

    func hello (val string) string {
     return "hello "+val;
    }

    func main() {
      web.Run(urls, "0.0.0.0:9999");
    }

## About

web.go was written by Michael Hoisie. Follow me on [Twitter](http://www.twitter.com/hoisie)!

