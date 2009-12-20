# web.go

web.go is the simplest way to write web applications in the Go programming language. 

## Overview

web.go should be familiar to people who've developed websites with higher-level web frameworks like sinatra, pylons, or web.py. It is designed to be a lightweight web framework that doesn't impose too much scaffolding on the code. Some features include:

* routing to url handlers based on regular expressions
* helper methods for rendering templates
* web applications are compiled to native code, which means very fast page render times (order-of-magnitude improvement over python or ruby frameworks)

Future releases will support:

* fcgi, scgi, and proxying support
* executing route handlers with goroutines for multicore systems
* ability to use asynchronous handlers (for long-polling)

## Installation

1. Make sure you have the latest Go sources (hg sync in the go tree), and your environment is set up correctly: $GOROOT, $GOARCH, $GOBIN, etc.
2. Checkout the code
3. cd web.go && make install

## Example
    
    package main
    
    import (
        "fmt"
        "web"
    )
    
    func hello(val string) string { 
        return fmt.Sprintf("hello %s", val) 
    }
    
    
    func main() {
        web.Get("/(.*)", hello)
        web.Run("0.0.0.0:9999")
    }


### Adding route handlers

We add a handler that matches the url path "/today". This will return the current url path. 

    package main
    
    import (
        "fmt"
        "time"
        "web"
    )

    func index() string {
        return "Welcome!"
    }

    func today() string {
        return fmt.Sprintf("The time is currently %s", time.LocalTime().Asctime())
    }
    
    
    func main() {
        web.Get("/today", today)
        web.Get("/", index)
        web.Run("0.0.0.0:9999")
    }
    
Then stop the application and recompile it . You can point your browser to http://localhost:9999/today to see the new route. 

## About

web.go was written by Michael Hoisie. Follow me on [Twitter](http://www.twitter.com/hoisie)!

