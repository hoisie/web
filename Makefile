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
	$(GC) ini.go
	$(GC) -I . webgo.go
	$(LD) -o webgo webgo.$(O)

install: wginstall


wginstall:
	! test -f $(GOBIN)/$(WEBGO) || chmod u+w $(GOBIN)/$(WEBGO)
	install -m 0755 $(WEBGO) $(GOBIN)
	
clean:	wgclean

wgclean:
	rm -rf webgo.8 webgo

format:
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w ini.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w web.go
	gofmt -spaces=true -tabindent=false -tabwidth=4 -w webgo.go
