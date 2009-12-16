# web.go

web.go is the simplest way to write web applications in the Go programming language. 

## Installation

1. Make sure you have Go installed and your environment is set up correctly: $GOROOT, $GOARCH, $GOBIN, etc.
2. Checkout the code
3. cd web.go && make install

## Example


### Creating a project 

 1. webgo create hello
 2. cd hello
 3. webgo serve default.ini
 4. open your browser to http://127.0.0.1:9999


### Adding route handlers

Modify hello.go to look like the following:

      package hello

      import (
        "time";
      )

      var Routes = map[string] interface {} {
        "/today" : today,
        "/(.*)" : hello,
      }

      func hello (val string) string {
       return "hello "+val;
      }

      func today () string {
       return "The time is currently "+time.LocalTime().Asctime();
      }

Then stop the application and re-run 'webgo serve default.ini'. You can point your browser to http://localhost:9999/today to see the new route. 

## About

web.go was written by Michael Hoisie. Follow me on [Twitter](http://www.twitter.com/hoisie)!

