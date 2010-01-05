# web.go

web.go is the simplest way to write web applications in the Go programming language. 

## Overview

web.go should be familiar to people who've developed websites with higher-level web frameworks like sinatra, pylons, or web.py. It is designed to be a lightweight web framework that doesn't impose any scaffolding on the user. Some features include:

* Routing to url handlers based on regular expressions
* User sessions
* Support for fastcgi and scgi
* Web applications are compiled to native code. This means very fast execution and page render speed ( benchmarks coming soon :)

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

For a quickstart guide, check out [web.go's home page](http://www.getwebgo.com)

There is also a [tutorial](http://www.getwebgo.com/tutorial)

If you use web.go, I'd greatly appreciate a quick message about what you're building with it. This will help me get a sense of usage patterns, and helps me focus development effors on features that people will actually use. 

## About

web.go was written by [Michael Hoisie](http://hoisie.com). 

Follow me on [Twitter](http://www.twitter.com/hoisie)!

