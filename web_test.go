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

type Test struct {
    method   string
    path     string
    body     string
    expected string
}

var tests = []Test{
    Test{"GET", "/", "", "index"},
    Test{"GET", "/echo/hello", "", "hello"},
    Test{"POST", "/post/echo/hello", "", "hello"},
    Test{"POST", "/post/echoparam/a", "a=hello", "hello"},
    //long url
    Test{"GET", "/echo/" + strings.Repeat("0123456789", 100), "", strings.Repeat("0123456789", 100)},
}

//initialize the routes
func init() {
    Get("/", func() string { return "index" })
    Get("/echo/(.*)", func(s string) string { return s })
    Get("/echo/(.*)", func(s string) string { return s })
    Post("/post/echo/(.*)", func(s string) string { return s })
    Post("/post/echoparam/(.*)", func(ctx *Context, name string) string { return ctx.Request.Form[name][0] })
}

func buildTestRequest(method string, path string, body string, headers map[string]string) *Request {
    host := "127.0.0.1"
    port := "80"
    rawurl := "http://" + host + ":" + port + path
    url, _ := http.ParseURL(rawurl)

    proto := "HTTP/1.1"
    useragent := "web.go test framework"

    if method == "POST" {
        headers["Content-Length"] = fmt.Sprintf("%d", len(body))
        headers["Content-Type"] = "text/plain"
    }

    req := Request{Method: method,
        RawURL: rawurl,
        URL: url,
        Proto: proto,
        Host: host,
        UserAgent: useragent,
        Header: headers,
        Body: bytes.NewBufferString(body),
    }

    return &req
}

func TestRouting(t *testing.T) {
    for _, test := range (tests) {
        req := buildTestRequest(test.method, test.path, test.body, make(map[string]string))
        var buf bytes.Buffer
        c := connProxy{status: 0, headers: make(map[string]string), buf: &buf}
        routeHandler(req, &c)
        body := buf.String()

        if body != test.expected {
            t.Fatalf("expected %q got %q", test.expected, body)
        }
    }
}

func buildScgiFields(fields map[string]string) []byte {
    var buf bytes.Buffer
    for k, v := range (fields) {
        buf.WriteString(k)
        buf.Write([]byte{0})
        buf.WriteString(v)
        buf.Write([]byte{0})
    }

    return buf.Bytes()
}

func buildTestScgiRequest(method string, path string, body string, headers map[string]string) *bytes.Buffer {
    scgiHeaders := make(map[string]string)

    scgiHeaders["REQUEST_METHOD"] = method
    scgiHeaders["HTTP_HOST"] = "127.0.0.1"
    scgiHeaders["REQUEST_URI"] = path
    scgiHeaders["SERVER_PORT"] = "80"
    scgiHeaders["SERVER_PROTOCOL"] = "HTTP/1.1"
    scgiHeaders["USER_AGENT"] = "web.go test framework"

    if method == "POST" {
        headers["Content-Length"] = fmt.Sprintf("%d", len(body))
        headers["Content-Type"] = "text/plain"
    }

    f1 := buildScgiFields(scgiHeaders)
    fields := f1

    if len(headers) > 0 {
        f2 := buildScgiFields(headers)
        fields = bytes.Join([][]byte{f1, f2}, []byte{})
    }

    var buf bytes.Buffer

    //comma at the end
    clen := len(fields) + len(body) + 1
    fmt.Fprintf(&buf, "%d:", clen)
    buf.Write(fields)
    buf.WriteString(",")
    buf.WriteString(body)

    return &buf
}

func TestScgi(t *testing.T) {
    for _, test := range (tests) {
        req := buildTestScgiRequest(test.method, test.path, test.body, make(map[string]string))
        var output bytes.Buffer

        nb := tcpProxy{input: req, output: &output}
        handleScgiRequest(&nb)

        contents := output.String()
        body := contents[strings.Index(contents, "\r\n\r\n")+4:]

        if body != test.expected {
            t.Fatalf("Scgi expected %q got %q", test.expected, body)
        }
    }
}

func buildFcgiKeyValue(key string, val string) []byte {

    var buf bytes.Buffer

    if len(key) <= 127 && len(val) <= 127 {
        data := struct {
            A   uint8
            B   uint8
        }{uint8(len(key)), uint8(len(val))}
        binary.Write(&buf, binary.BigEndian, data)
    } else if len(key) <= 127 && len(val) > 127 {
        data := struct {
            A   uint8
            B   uint32
        }{uint8(len(key)), uint32(len(val)) | 1<<31}

        binary.Write(&buf, binary.BigEndian, data)
    }
    buf.Write(strings.Bytes(key))
    buf.Write(strings.Bytes(val))

    return buf.Bytes()
}

func buildTestFcgiRequest(method string, path string, body string, headers map[string]string) *bytes.Buffer {

    var req bytes.Buffer
    fcgiHeaders := make(map[string]string)

    fcgiHeaders["REQUEST_METHOD"] = method
    fcgiHeaders["HTTP_HOST"] = "127.0.0.1"
    fcgiHeaders["REQUEST_URI"] = path
    fcgiHeaders["SERVER_PORT"] = "80"
    fcgiHeaders["SERVER_PROTOCOL"] = "HTTP/1.1"
    fcgiHeaders["USER_AGENT"] = "web.go test framework"

    if method == "POST" {
        fcgiHeaders["Content-Length"] = fmt.Sprintf("%d", len(body))
        fcgiHeaders["Content-Type"] = "text/plain"
    }

    // add the begin request
    req.Write(newFcgiRecord(FcgiBeginRequest, 0, make([]byte, 8)))

    var buf bytes.Buffer
    for k, v := range (fcgiHeaders) {
        kv := buildFcgiKeyValue(k, v)
        buf.Write(kv)
    }

    //add the params request
    req.Write(newFcgiRecord(FcgiParams, 0, buf.Bytes()))

    //send the body
    if len(body) > 0 {
        req.Write(newFcgiRecord(FcgiStdin, 0, strings.Bytes(body)))
    }

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
    for _, test := range (tests) {
        req := buildTestFcgiRequest(test.method, test.path, test.body, make(map[string]string))
        var output bytes.Buffer
        nb := tcpProxy{input: req, output: &output}
        handleFcgiConnection(&nb)
        contents := getFcgiOutput(&output).String()
        body := contents[strings.Index(contents, "\r\n\r\n")+4:]

        if body != test.expected {
            t.Fatalf("Fcgi exected %q got %q", test.expected, body)
        }
    }
}
