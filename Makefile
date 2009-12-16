# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.$(GOARCH)

TARG=web
WEBGO=webgo
GOFILES=\
	web.go\

include $(GOROOT)/src/Make.pkg

all:	wg

wg:
	8g ini.go
	8g -I . webgo.go
	8l -o webgo webgo.8
	chmod +x webgo

install: wginstall


wginstall:
	! test -f "$(GOBIN)"/$(WEBGO) || chmod u+w "$(GOBIN)"/$(WEBGO)
	cp $(WEBGO) "$(GOBIN)"/$(WEBGO)
	
clean:	wgclean

wgclean:
	rm -rf webgo.8 webgo

format:
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w ini.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w web.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w webgo.go
