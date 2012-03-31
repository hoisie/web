package web

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cgi"
	"strings"
)

const (
	fcgiBeginRequest = iota + 1
	fcgiAbortRequest
	fcgiEndRequest
	fcgiParams
	fcgiStdin
	fcgiStdout
	fcgiStderr
	fcgiData
	fcgiGetValues
	fcgiGetValuesResult
	fcgiUnknownType
	fcgiMaxType = fcgiUnknownType
)

const (
	fcgiRequestComplete = iota
	fcgiCantMpxConn
	fcgiOverloaded
	fcgiUnknownRole
)

type fcgiHeader struct {
	Version       uint8
	Type          uint8
	RequestId     uint16
	ContentLength uint16
	PaddingLength uint8
	Reserved      uint8
}

func (h fcgiHeader) bytes() []byte {
	order := binary.BigEndian
	buf := make([]byte, 8)
	buf[0] = h.Version
	buf[1] = h.Type
	order.PutUint16(buf[2:4], h.RequestId)
	order.PutUint16(buf[4:6], h.ContentLength)
	buf[6] = h.PaddingLength
	buf[7] = h.Reserved
	return buf
}

func newFcgiRecord(typ int, requestId int, data []byte) []byte {
	var record bytes.Buffer
	l := len(data)
	// round to the nearest 8
	padding := make([]byte, uint8(-l&7))
	hdr := fcgiHeader{
		Version:       1,
		Type:          uint8(typ),
		RequestId:     uint16(requestId),
		ContentLength: uint16(l),
		PaddingLength: uint8(len(padding)),
	}

	//write the header
	record.Write(hdr.bytes())
	record.Write(data)
	record.Write(padding)

	return record.Bytes()
}

type fcgiEndReq struct {
	appStatus      uint32
	protocolStatus uint8
	reserved       [3]uint8
}

func (er fcgiEndReq) bytes() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf, er.appStatus)
	buf[4] = er.protocolStatus
	return buf
}

type fcgiConn struct {
	requestId    uint16
	req          *http.Request
	fd           io.ReadWriteCloser
	headers      http.Header
	wroteHeaders bool
}

func (conn *fcgiConn) fcgiWrite(data []byte) (err error) {
	l := len(data)
	// round to the nearest 8
	padding := make([]byte, uint8(-l&7))
	hdr := fcgiHeader{
		Version:       1,
		Type:          fcgiStdout,
		RequestId:     conn.requestId,
		ContentLength: uint16(l),
		PaddingLength: uint8(len(padding)),
	}

	//write the header
	hdrBytes := hdr.bytes()
	_, err = conn.fd.Write(hdrBytes)

	if err != nil {
		return err
	}

	_, err = conn.fd.Write(data)
	if err != nil {
		return err
	}

	_, err = conn.fd.Write(padding)
	if err != nil {
		return err
	}

	return err
}

func (conn *fcgiConn) Write(data []byte) (n int, err error) {
	if !conn.wroteHeaders {
		conn.WriteHeader(200)
	}

	if conn.req.Method == "HEAD" {
		return 0, errors.New("Body Not Allowed")
	}
	err = conn.fcgiWrite(data)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (conn *fcgiConn) WriteHeader(status int) {
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
		conn.fcgiWrite(buf.Bytes())
	}
}

func (conn *fcgiConn) Header() http.Header {
	return conn.headers
}

func (conn *fcgiConn) complete() {
	content := fcgiEndReq{appStatus: 200, protocolStatus: fcgiRequestComplete}.bytes()
	l := len(content)

	hdr := fcgiHeader{
		Version:       1,
		Type:          fcgiEndRequest,
		RequestId:     uint16(conn.requestId),
		ContentLength: uint16(l),
		PaddingLength: 0,
	}

	conn.fd.Write(hdr.bytes())
	conn.fd.Write(content)
	conn.fd.Close()
}

func (conn *fcgiConn) Close() {}

func readFcgiParamSize(data []byte, index int) (int, int) {

	var size int
	var shift = 0

	if data[index]>>7 == 0 {
		size = int(data[index])
		shift = 1
	} else {
		var s uint32
		binary.Read(bytes.NewBuffer(data[index:index+4]), binary.BigEndian, &s)
		s ^= 1 << 31
		size = int(s)
		shift = 4
	}
	return size, shift

}

//read the fcgi parameters contained in data, and store them in storage
func readFcgiParams(data []byte, storage map[string]string) {
	for idx := 0; len(data) > idx; {
		keySize, shift := readFcgiParamSize(data, idx)
		idx += shift
		valSize, shift := readFcgiParamSize(data, idx)
		idx += shift
		key := data[idx : idx+keySize]
		idx += keySize
		val := data[idx : idx+valSize]
		idx += valSize
		storage[string(key)] = string(val)
	}
}

func (s *Server) handleFcgiConnection(fd io.ReadWriteCloser) {
	br := bufio.NewReader(fd)
	var req *http.Request
	var fc *fcgiConn
	var body bytes.Buffer
	headers := map[string]string{}

	for {
		var h fcgiHeader
		err := binary.Read(br, binary.BigEndian, &h)
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Logger.Println("FCGI Error", err.Error())
			break
		}
		content := make([]byte, h.ContentLength)
		_, err = io.ReadFull(br, content)
		if err != nil {
			s.Logger.Println("FCGI Error", err.Error())
			break
		}

		//read padding
		if h.PaddingLength > 0 {
			padding := make([]byte, h.PaddingLength)
			_, err = io.ReadFull(br, padding)
			if err != nil {
				s.Logger.Println("FCGI Error", err.Error())
				break
			}
		}

		switch h.Type {
		case fcgiBeginRequest:
			fc = &fcgiConn{h.RequestId, req, fd, make(map[string][]string), false}

		case fcgiParams:
			if h.ContentLength > 0 {
				readFcgiParams(content, headers)
			}
		case fcgiStdin:
			if h.ContentLength > 0 {
				body.Write(content)
			} else if h.ContentLength == 0 {
				req, _ = cgi.RequestFromMap(headers)
				req.Body = ioutil.NopCloser(&body)
				fc.req = req
				s.routeHandler(req, fc)
				//we close the connection after processing
				//TODO: is there a way to keep it open for future requests?
				fc.complete()
				return
			}
		case fcgiData:
			if h.ContentLength > 0 {
				body.Write(content)
			}
		case fcgiAbortRequest:
		}
	}
}

func (s *Server) listenAndServeFcgi(addr string) error {
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
		s.Logger.Println("FCGI listen error", err.Error())
		return err
	}
	for {
		fd, err := l.Accept()
		if err != nil {
			s.Logger.Println("FCGI accept error", err.Error())
			break
		}
		go s.handleFcgiConnection(fd)
	}
	return nil
}
