package web

import (
    "bytes"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "strconv"
)

type scgiConn struct {
    fd           io.ReadWriteCloser
    headers      map[string][]string
    wroteHeaders bool
}

func (conn *scgiConn) StartResponse(status int) {
    var buf bytes.Buffer
    text := statusText[status]
    fmt.Fprintf(&buf, "HTTP/1.1 %d %s\r\n", status, text)
    conn.fd.Write(buf.Bytes())
}

func (conn *scgiConn) SetHeader(hdr string, val string, unique bool) {
    if _, contains := conn.headers[hdr]; !contains {
        conn.headers[hdr] = []string{val}
        return
    }

    if unique {
        //just overwrite the first value
        conn.headers[hdr][0] = val
    } else {
        newHeaders := make([]string, len(conn.headers)+1)
        copy(newHeaders, conn.headers[hdr])
        newHeaders[len(newHeaders)-1] = val
        conn.headers[hdr] = newHeaders
    }
}

func (conn *scgiConn) Write(data []byte) (n int, err os.Error) {
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
    return conn.fd.Write(data)
}

func (conn *scgiConn) Close() { conn.fd.Close() }

func readScgiRequest(buf *bytes.Buffer) (*Request, os.Error) {
    headers := make(map[string]string)

    data := buf.Bytes()
    var clen int

    colon := bytes.IndexByte(data, ':')
    data = data[colon+1:]
    var err os.Error
    //find the CONTENT_LENGTH

    clfields := bytes.Split(data, []byte{0}, 3)
    if len(clfields) != 3 {
        return nil, os.NewError("Invalid SCGI Request -- no fields")
    }

    clfields = clfields[0:2]
    if string(clfields[0]) != "CONTENT_LENGTH" {
        return nil, os.NewError("Invalid SCGI Request -- expecing CONTENT_LENGTH")
    }

    if clen, err = strconv.Atoi(string(clfields[1])); err != nil {
        return nil, os.NewError("Invalid SCGI Request -- invalid CONTENT_LENGTH field")
    }

    content := data[len(data)-clen:]

    fields := bytes.Split(data[0:len(data)-clen], []byte{0}, 0)

    for i := 0; i < len(fields)-1; i += 2 {
        key := string(fields[i])
        value := string(fields[i+1])
        headers[key] = value
    }

    body := bytes.NewBuffer(content)
    req := newRequestCgi(headers, body)

    return req, nil
}

func handleScgiRequest(fd io.ReadWriteCloser) {
    var buf bytes.Buffer
    var tmp [1024]byte
    n, err := fd.Read(&tmp)
    if err != nil || n == 0 {
        return
    }

    colonPos := bytes.IndexByte(tmp[0:], ':')

    read := n
    length, _ := strconv.Atoi(string(tmp[0:colonPos]))
    buf.Write(tmp[0:n])

    for read < length {
        n, err := fd.Read(&tmp)
        if err != nil || n == 0 {
            break
        }

        buf.Write(tmp[0:n])
        read += n
    }

    req, err := readScgiRequest(&buf)

    if err != nil {
        log.Stderrf("SCGI read error", err.String())
        return
    }

    sc := scgiConn{fd, make(map[string][]string), false}
    routeHandler(req, &sc)
    fd.Close()
}

func listenAndServeScgi(addr string) {
    l, err := net.Listen("tcp", addr)
    if err != nil {
        log.Stderrf("SCGI listen error", err.String())
        return
    }

    for {
        fd, err := l.Accept()
        if err != nil {
            log.Stderrf("SCGI accept error", err.String())
            break
        }
        go handleScgiRequest(fd)

    }
}
