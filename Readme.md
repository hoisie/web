# web.go

web.go is the simplest way to write web applications in the Go programming language. 

See [www.getwebgo.com](http://www.getwebgo.com) for complete documentation. 

## Overview

web.go should be familiar to people who've developed websites with higher-level web frameworks like sinatra, pylons, or web.py. It is designed to be a lightweight web framework that doesn't impose any scaffolding on the user. Some features include:

* Routing to url handlers based on regular expressions
* Support for user sessions
* Web applications are compiled to native code. This means very fast execution and page render speed ( benchmarks coming soon :)

To use web.go, simply install it and import the "web" package. You then need to map url routes to function handlers using web.Get, web.Post, etc.. Finally, you need to call web.Run with the address of the host. (see the example below)

## Installation

1. Make sure you have the a working Go environment. See the [install instructions](http://golang.org/doc/install.html)
2. git clone git://github.com/hoisie/web.go.git
3. cd web.go && make install

## Example
    
    package main
    
    import (
        "web"
    )
    
    func hello(val string) string { return "hello " + val } 
    
    func main() {
        web.Get("/(.*)", hello)
        web.Run("0.0.0.0:9999")
    }


To run the application, put the code in a file called hello.go and run:

    8g hello.go && 8l -o hello hello.8 && ./hello

You can point your browser to http://localhost:9999/world . 

## Documentation

More documentation, including a tutorial and API docs, is available on [web.go's home page][http://www.getwebgo.com]

## About

web.go was written by [Michael Hoisie](http://hoisie.com). 

Follow me on [Twitter](http://www.twitter.com/hoisie)!

