package web

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cgi"
	"strconv"
	"strings"
)

type scgiConn struct {
	fd           io.ReadWriteCloser
	req          *http.Request
	headers      http.Header
	wroteHeaders bool
}

func (conn *scgiConn) WriteHeader(status int) {
	if !conn.wroteHeaders {
		conn.wroteHeaders = true

		var buf bytes.Buffer
		text := statusText[status]

		fmt.Fprintf(&buf, "HTTP/1.1 %d %s\r\n", status, text)

		for k, v := range conn.headers {
			for _, i := range v {
				buf.WriteString(k + ": " + i + "\r\n")
			}
		}

		buf.WriteString("\r\n")
		conn.fd.Write(buf.Bytes())
	}
}

func (conn *scgiConn) Header() http.Header {
	return conn.headers
}

func (conn *scgiConn) Write(data []byte) (n int, err error) {
	if !conn.wroteHeaders {
		conn.WriteHeader(200)
	}

	if conn.req.Method == "HEAD" {
		return 0, errors.New("Body Not Allowed")
	}

	return conn.fd.Write(data)
}

func (conn *scgiConn) Close() { conn.fd.Close() }

func (conn *scgiConn) finishRequest() error {
	var buf bytes.Buffer
	if !conn.wroteHeaders {
		conn.wroteHeaders = true
		for k, v := range conn.headers {
			for _, i := range v {
				buf.WriteString(k + ": " + i + "\r\n")
			}
		}

		buf.WriteString("\r\n")
		conn.fd.Write(buf.Bytes())
	}
	return nil
}

func readScgiRequest(buf *bytes.Buffer) (*http.Request, error) {
	headers := map[string]string{}

	data := buf.Bytes()
	var clen int

	colon := bytes.IndexByte(data, ':')
	data = data[colon+1:]
	var err error
	//find the CONTENT_LENGTH

	clfields := bytes.SplitN(data, []byte{0}, 3)
	if len(clfields) != 3 {
		return nil, errors.New("Invalid SCGI Request -- no fields")
	}

	clfields = clfields[0:2]
	if string(clfields[0]) != "CONTENT_LENGTH" {
		return nil, errors.New("Invalid SCGI Request -- expecting CONTENT_LENGTH")
	}

	if clen, err = strconv.Atoi(string(clfields[1])); err != nil {
		return nil, errors.New("Invalid SCGI Request -- invalid CONTENT_LENGTH field")
	}

	content := data[len(data)-clen:]

	fields := bytes.Split(data[0:len(data)-clen], []byte{0})

	for i := 0; i < len(fields)-1; i += 2 {
		key := string(fields[i])
		value := string(fields[i+1])
		headers[key] = value
	}

	body := bytes.NewBuffer(content)

	req, _ := cgi.RequestFromMap(headers)
	req.Body = ioutil.NopCloser(body)

	return req, nil
}

func (s *Server) handleScgiRequest(fd io.ReadWriteCloser) {
	var buf bytes.Buffer
	tmp := make([]byte, 1024)
	n, err := fd.Read(tmp)
	if err != nil || n == 0 {
		return
	}

	colonPos := bytes.IndexByte(tmp[0:], ':')

	read := n
	length, _ := strconv.Atoi(string(tmp[0:colonPos]))
	buf.Write(tmp[0:n])

	for read < length {
		n, err := fd.Read(tmp)
		if err != nil || n == 0 {
			break
		}

		buf.Write(tmp[0:n])
		read += n
	}

	req, err := readScgiRequest(&buf)
	if err != nil {
		s.Logger.Println("SCGI read error", err.Error())
		return
	}

	sc := scgiConn{fd, req, make(map[string][]string), false}
	s.routeHandler(req, &sc)
	sc.finishRequest()
	fd.Close()
}

func (s *Server) listenAndServeScgi(addr string) error {

	var l net.Listener
	var err error

	//if the path begins with a "/", assume it's a unix address
	if strings.HasPrefix(addr, "/") {
		l, err = net.Listen("unix", addr)
	} else {
		l, err = net.Listen("tcp", addr)
	}

	//save the listener so it can be closed
	s.l = l

	if err != nil {
		s.Logger.Println("SCGI listen error", err.Error())
		return err
	}

	for {
		fd, err := l.Accept()
		if err != nil {
			s.Logger.Println("SCGI accept error", err.Error())
			return err
		}
		go s.handleScgiRequest(fd)
	}
	return nil
}
