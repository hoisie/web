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
type bufferedConn struct {
    status  int
    headers map[string][]string
    buf     *bytes.Buffer
}

func (c *bufferedConn) StartResponse(status int) {
    c.status = status
}

func (conn *bufferedConn) SetHeader(hdr string, val string, unique bool) {
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

func (c *bufferedConn) WriteString(content string) {
    c.buf.WriteString(content)
}

func (c *bufferedConn) Write(content []byte) (n int, err os.Error) {
    c.buf.Write(content)
    return n, nil
}

func (c *bufferedConn) Close() {}

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
    Test{"GET", "/multiecho/a/b/c/d", "", "abcd"},
    Test{"POST", "/post/echo/hello", "", "hello"},
    Test{"POST", "/post/echoparam/a", "a=hello", "hello"},
    //long url
    Test{"GET", "/echo/" + strings.Repeat("0123456789", 100), "", strings.Repeat("0123456789", 100)},
}

//initialize the routes
func init() {
    Get("/", func() string { return "index" })
    Get("/echo/(.*)", func(s string) string { return s })
    Get("/multiecho/(.*)/(.*)/(.*)/(.*)", func(a, b, c, d string) string { return a + b + c + d })
    Post("/post/echo/(.*)", func(s string) string { return s })
    Post("/post/echoparam/(.*)", func(ctx *Context, name string) string { return ctx.Request.Params[name][0] })

    Post("/session/set/(.+)/(.+)", func(ctx *Context, name string, val string) string {
        ctx.Session.Data[name] = val
        return ""
    })

    Get("/session/get/(.*)", func(ctx *Context, name string) string { return ctx.Session.Data[name].(string) })

}

func buildTestRequest(method string, path string, body string, headers map[string]string) *Request {
    host := "127.0.0.1"
    port := "80"
    rawurl := "http://" + host + ":" + port + path
    url, _ := http.ParseURL(rawurl)

    proto := "HTTP/1.1"
    useragent := "web.go test framework"

    if headers == nil {
        headers = map[string]string{}
    }

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
        Headers: headers,
        Body: bytes.NewBufferString(body),
    }

    return &req
}

func TestRouting(t *testing.T) {
    for _, test := range (tests) {
        req := buildTestRequest(test.method, test.path, test.body, make(map[string]string))
        var buf bytes.Buffer
        c := bufferedConn{status: 0, headers: make(map[string][]string), buf: &buf}
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

func buildTestFcgiRequest(method string, path string, bodychunks []string, headers map[string]string) *bytes.Buffer {

    var req bytes.Buffer
    fcgiHeaders := make(map[string]string)

    fcgiHeaders["REQUEST_METHOD"] = method
    fcgiHeaders["HTTP_HOST"] = "127.0.0.1"
    fcgiHeaders["REQUEST_URI"] = path
    fcgiHeaders["SERVER_PORT"] = "80"
    fcgiHeaders["SERVER_PROTOCOL"] = "HTTP/1.1"
    fcgiHeaders["USER_AGENT"] = "web.go test framework"

    bodylength := 0
    for _, s := range (bodychunks) {
        bodylength += len(s)
    }

    if method == "POST" {
        fcgiHeaders["Content-Length"] = fmt.Sprintf("%d", bodylength)
        fcgiHeaders["Content-Type"] = "text/plain"
    }

    // add the begin request
    req.Write(newFcgiRecord(fcgiBeginRequest, 0, make([]byte, 8)))

    var buf bytes.Buffer
    for k, v := range (fcgiHeaders) {
        kv := buildFcgiKeyValue(k, v)
        buf.Write(kv)
    }

    //add the params record
    req.Write(newFcgiRecord(fcgiParams, 0, buf.Bytes()))

    //add the end-of-params record
    req.Write(newFcgiRecord(fcgiParams, 0, []byte{}))

    //send the body
    for _, s := range (bodychunks) {
        if len(s) > 0 {
            req.Write(newFcgiRecord(fcgiStdin, 0, strings.Bytes(s)))
        }
    }

    //add the end-of-stdin record
    req.Write(newFcgiRecord(fcgiStdin, 0, []byte{}))

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

        if h.Type == fcgiStdout {
            output.Write(content)
        }
    }

    return &output
}

func TestFcgi(t *testing.T) {
    for _, test := range (tests) {
        req := buildTestFcgiRequest(test.method, test.path, []string{test.body}, make(map[string]string))
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


func TestFcgiChucks(t *testing.T) {
    //split up an fcgi request
    bodychunks := []string{`a=12&b=`, strings.Repeat("1234567890", 200)}

    req := buildTestFcgiRequest("POST", "/post/echoparam/b", bodychunks, make(map[string]string))
    var output bytes.Buffer
    nb := tcpProxy{input: req, output: &output}
    handleFcgiConnection(&nb)
    contents := getFcgiOutput(&output).String()
    body := contents[strings.Index(contents, "\r\n\r\n")+4:]

    if body != strings.Repeat("1234567890", 200) {
        t.Fatalf("Fcgi chunks test failed")
    }
}

func TestSession(t *testing.T) {

	//set a=1 i the session
    setreq := buildTestRequest("POST", "/session/set/a/1", "", nil)
    
	var b1 bytes.Buffer;
    c1 := bufferedConn{headers: make(map[string][]string), buf: &b1}
    routeHandler(setreq, &c1)
    
    cookie := c1.headers["Set-Cookie"][0]
    if strings.HasPrefix(cookie, "wgosession=") {
    	i := strings.Index(cookie, ";");
    	cookie = cookie[0:i]
    }
    
    //pass the session cookie
    getreq := buildTestRequest("GET", "/session/get/a", "", map[string]string { "Cookie": cookie} )
    var b2 bytes.Buffer
    c2 := bufferedConn{headers: make(map[string][]string), buf: &b2}
    routeHandler(getreq, &c2)
    body := b2.String()
   	if body != "1" {
        t.Fatalf("Session test failed")
    }
}
