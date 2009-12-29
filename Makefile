# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.$(GOARCH)

TARG=web
GOFILES=\
	fcgi.go\
	scgi.go\
	servefile.go\
	web.go\

include $(GOROOT)/src/Make.pkg

format:
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w fcgi.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w scgi.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w servefile.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w web.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w web_test.go
