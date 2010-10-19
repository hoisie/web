include $(GOROOT)/src/Make.inc

TARG=web
GOFMT=gofmt -spaces=true -tabindent=false -tabwidth=4

GOFILES=\
	fcgi.go\
	request.go\
	scgi.go\
	servefile.go\
	status.go\
	web.go\

include $(GOROOT)/src/Make.pkg

format:
	${GOFMT} -w fcgi.go
	${GOFMT} -w request.go
	${GOFMT} -w scgi.go
	${GOFMT} -w servefile.go
	${GOFMT} -w status.go
	${GOFMT} -w web.go
	${GOFMT} -w web_test.go
	${GOFMT} -w examples/arcchallenge.go
	${GOFMT} -w examples/hello.go
	${GOFMT} -w examples/multipart.go
	${GOFMT} -w examples/multiserver.go
	${GOFMT} -w examples/params.go
	${GOFMT} -w examples/logger.go
