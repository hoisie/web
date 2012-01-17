include $(GOROOT)/src/Make.inc

TARG=github.com/hoisie/web.go
GOFMT=gofmt -s -spaces=true -tabindent=false -tabwidth=4

GOFILES=\
	cookie.go\
	fcgi.go\
	request.go\
	scgi.go\
	servefile.go\
	status.go\
	web.go\

include $(GOROOT)/src/Make.pkg

format:
	${GOFMT} -w ${GOFILES}
	${GOFMT} -w web_test.go
	${GOFMT} -w examples/arcchallenge.go
	${GOFMT} -w examples/hello.go
	${GOFMT} -w examples/methodhandler.go
	${GOFMT} -w examples/multipart.go
	${GOFMT} -w examples/multiserver.go
	${GOFMT} -w examples/params.go
	${GOFMT} -w examples/logger.go
