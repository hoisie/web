package web

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "http"
    "os"
    "strings"
    "testing"
)

// this implements the web.Connection interface. It's useful to test routing handlers
type connProxy struct {
    status  int
    headers map[string]string
    buf     *bytes.Buffer
}

func (c *connProxy) StartResponse(status int) { c.status = status }

func (c *connProxy) SetHeader(hdr string, val string) {
    c.headers[hdr] = val
}

func (c *connProxy) WriteString(content string) {
    c.buf.WriteString(content)
}

func (c *connProxy) Write(content []byte) (n int, err os.Error) {
    c.buf.Write(content)
    return n, nil
}

func (c *connProxy) Close() {}

//this implements io.ReadWriteCloser, which means it can be passed around as a tcp connection
type tcpProxy struct {
    input  *bytes.Buffer
    output *bytes.Buffer
}

func (buf *tcpProxy) Write(p []uint8) (n int, err os.Error) {
    return buf.output.Write(p)
}

func (buf *tcpProxy) Read(p []byte) (n int, err os.Error) {
    return buf.input.Read(p)
}

func (buf *tcpProxy) Close() os.Error { return nil }


func buildTestRequest(method string, path string, headers map[string]string) *Request {
    host := "127.0.0.1"
    port := "80"
    rawurl := "http://" + host + ":" + port + path
    url, _ := http.ParseURL(rawurl)

    proto := "HTTP/1.1"
    useragent := "web.go test framework"

    req := Request{Method: method,
        RawURL: rawurl,
        URL: url,
        Proto: proto,
        Host: host,
        UserAgent: useragent,
        Header: headers,
    }

    return &req
}

func TestRouting(t *testing.T) {
    // set up echo route
    Get("/(.*)", func(s string) string { return s })

    req := buildTestRequest("GET", "/hello", make(map[string]string))
    var buf bytes.Buffer
    c := connProxy{status: 0, headers: make(map[string]string), buf: &buf}

    routeHandler(req, &c)

    body := buf.String()

    if body != "hello" {
        t.Fatal("Scgi test failed")
    }

}

func scgiFields(fields map[string]string) []byte {
    var buf bytes.Buffer
    for k, v := range (fields) {
        buf.WriteString(k)
        buf.Write([]byte{0})
        buf.WriteString(v)
        buf.Write([]byte{0})
    }

    return buf.Bytes()
}

func buildTestScgiRequest(method string, path string, headers map[string]string) *bytes.Buffer {

    scgiHeaders := make(map[string]string)

    scgiHeaders["REQUEST_METHOD"] = method
    scgiHeaders["CONTENT_LENGTH"] = "0"
    scgiHeaders["HTTP_HOST"] = "127.0.0.1"
    scgiHeaders["REQUEST_URI"] = path
    scgiHeaders["SERVER_PORT"] = "80"
    scgiHeaders["SERVER_PROTOCOL"] = "HTTP/1.1"
    scgiHeaders["USER_AGENT"] = "web.go test framework"

    f1 := scgiFields(scgiHeaders)
    fields := f1

    if len(headers) > 0 {
        f2 := scgiFields(headers)
        fields = bytes.Join([][]byte{f1, f2}, []byte{})
    }

    var buf bytes.Buffer

    //comma at the end
    clen := len(fields) + 1
    fmt.Fprintf(&buf, "%d:", clen)
    buf.Write(fields)
    buf.WriteString(",")

    return &buf
}

func TestScgi(t *testing.T) {

    req := buildTestScgiRequest("GET", "/hello", make(map[string]string))
    var output bytes.Buffer

    nb := tcpProxy{input: req, output: &output}
    handleScgiRequest(&nb)

    contents := output.String()
    body := contents[strings.Index(contents, "\r\n\r\n")+4:]

    if body != "hello" {
        t.Fatal("Scgi test failed")
    }
}

func encodeFcgiKeyValue(key string, val string) []byte {

    var buf bytes.Buffer

    if len(key) <= 127 && len(val) <= 127 {
        data := struct {
            A   uint8
            B   uint8
        }{uint8(len(key)), uint8(len(val))}
        binary.Write(&buf, binary.BigEndian, data)
    }

    buf.Write(strings.Bytes(key))
    buf.Write(strings.Bytes(val))

    return buf.Bytes()
}

func buildTestFcgiRequest(method string, path string, headers map[string]string) *bytes.Buffer {

    var req bytes.Buffer
    fcgiHeaders := make(map[string]string)

    fcgiHeaders["REQUEST_METHOD"] = method
    fcgiHeaders["HTTP_HOST"] = "127.0.0.1"
    fcgiHeaders["REQUEST_URI"] = path
    fcgiHeaders["SERVER_PORT"] = "80"
    fcgiHeaders["SERVER_PROTOCOL"] = "HTTP/1.1"
    fcgiHeaders["USER_AGENT"] = "web.go test framework"

    // add the begin request
    req.Write(newFcgiRecord(FcgiBeginRequest, 0, make([]byte, 8)))

    var buf bytes.Buffer
    for k, v := range (fcgiHeaders) {
        kv := encodeFcgiKeyValue(k, v)
        buf.Write(kv)
    }

    //add the params request
    req.Write(newFcgiRecord(FcgiParams, 0, buf.Bytes()))

    //add the stdin request
    req.Write(newFcgiRecord(FcgiStdin, 0, []byte{}))

    return &req
}

func getFcgiOutput(br *bytes.Buffer) *bytes.Buffer {
    var output bytes.Buffer
    for {
        var h fcgiHeader
        err := binary.Read(br, binary.BigEndian, &h)
        if err == os.EOF {
            break
        }

        content := make([]byte, h.ContentLength)
        br.Read(content)

        //read padding
        if h.PaddingLength > 0 {
            padding := make([]byte, h.PaddingLength)
            br.Read(padding)
        }

        if h.Type == FcgiStdout {
            output.Write(content)
        }
    }

    return &output
}

func TestFcgi(t *testing.T) {
    req := buildTestFcgiRequest("GET", "/hello", make(map[string]string))
    var output bytes.Buffer
    nb := tcpProxy{input: req, output: &output}
    handleFcgiConnection(&nb)
    contents := getFcgiOutput(&output).String()
    body := contents[strings.Index(contents, "\r\n\r\n")+4:]

    if body != "hello" {
        t.Fatal("Fcgi test failed")
    }
}
