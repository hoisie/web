GOFMT=gofmt -s -tabs=false -tabwidth=4

GOFILES=\
	fcgi.go\
	scgi.go\
	status.go\
	web.go\

format:
	${GOFMT} -w ${GOFILES}
	${GOFMT} -w web_test.go
	${GOFMT} -w examples/arcchallenge.go
	${GOFMT} -w examples/hello.go
	${GOFMT} -w examples/multipart.go
	${GOFMT} -w examples/multiserver.go
	${GOFMT} -w examples/params.go
	${GOFMT} -w examples/logger.go
	${GOFMT} -w examples/tls.go
