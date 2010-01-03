# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.$(GOARCH)

TARG=web
GOFMT=gofmt -spaces=true -tabindent=false -tabwidth=4

GOFILES=\
	fcgi.go\
	request.go\
	scgi.go\
	servefile.go\
	web.go\

include $(GOROOT)/src/Make.pkg

format:
	${GOFMT} -w fcgi.go
	${GOFMT} -w request.go
	${GOFMT} -w scgi.go
	${GOFMT} -w servefile.go
	${GOFMT} -w web.go
	${GOFMT} -w web_test.go
	${GOFMT} -w examples/hello.go
	${GOFMT} -w examples/arcchallenge.go
