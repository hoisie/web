GOFMT=gofmt -s -tabs=false -tabwidth=4

GOFILES=$(wildcard *.go **/*.go)

format:
	${GOFMT} -w ${GOFILES}
