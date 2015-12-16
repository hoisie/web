# web.go

web.go is the simplest way to write web applications in the Go programming language. It's ideal for writing simple, performant backend web services. 

## Overview

web.go should be familiar to people who've developed websites with higher-level web frameworks like sinatra or web.py. It is designed to be a lightweight web framework that doesn't impose any scaffolding on the user. Some features include:

* Routing to url handlers based on regular expressions
* Secure cookies
* Support for fastcgi and scgi
* Web applications are compiled to native code. This means very fast execution and page render speed
* Efficiently serving static files

## Installation

Make sure you have the a working Go environment. See the [install instructions](http://golang.org/doc/install.html). web.go targets the Go `release` branch.

To install web.go, simply run:

    go get github.com/shivakumargn/web

To compile it from source:

    git clone git://github.com/shivakumargn/web.git
    cd web && go build

## Example
(Ignore the ones in the examples folder. These are not adapted from hoisie/web. Use below example instead)

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/shivakumargn/web"
)

var logger = log.New(os.Stdout, "", log.Lshortfile|log.Ldate|log.Ltime)

func Greet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	action_name := vars["action_name"]
	fmt.Fprintf(w, action_name+":hi")
	w.WriteHeader(http.StatusOK)
	return
}

func main() {
	
	addr := "0.0.0.0:9998"
	logger.Println("starting http server at -", addr)

	s := web.NewServer()

	// Access the gorilla/mux router as s.Router(). Use gorilla/mux documentation for possible options for the router !
	s.Router().HandleFunc("/api/v1/greet/{action_name:[A-Za-z0-9_]+}", Greet).Methods("GET", "POST")

	go s.Run(addr)
	select {}
}
```

To run the application, put the code in a file called hello.go and run:

    go run hello.go
    
You can point your browser to http://localhost:9998/greet/hello_world

### Access url details (including parameters)

The gorilla/mux router is used as the underlying router. Server created as web.NewServer() provides Router() method that can be used to access and configure the underlying gorilla/mux router.

## About

This web.go is adapted from github.com/hoisie/web to work with gorilla/mux. 


