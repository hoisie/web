package web

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "http"
    "os"
    "strconv"
    "strings"
    "testing"
)

//this implements io.ReadWriteCloser, which means it can be passed around as a tcp connection
type tcpBuffer struct {
    input  *bytes.Buffer
    output *bytes.Buffer
}

func (buf *tcpBuffer) Write(p []uint8) (n int, err os.Error) {
    return buf.output.Write(p)
}

func (buf *tcpBuffer) Read(p []byte) (n int, err os.Error) {
    return buf.input.Read(p)
}

func (buf *tcpBuffer) Close() os.Error { return nil }

type testResponse struct {
    statusCode int
    status     string
    body       string
    headers    map[string][]string
    cookies    map[string]string
}

func buildTestResponse(buf *bytes.Buffer) *testResponse {

    response := testResponse{headers: make(map[string][]string), cookies: make(map[string]string)}
    s := buf.String()
    contents := strings.Split(s, "\r\n\r\n", 2)

    header := contents[0]
    response.body = contents[1]

    headers := strings.Split(header, "\r\n", 0)

    statusParts := strings.Split(headers[0], " ", 3)
    response.statusCode, _ = strconv.Atoi(statusParts[1])

    for _, h := range (headers[1:]) {
        split := strings.Split(h, ":", 2)
        name := strings.TrimSpace(split[0])
        value := strings.TrimSpace(split[1])
        if _, ok := response.headers[name]; !ok {
            response.headers[name] = []string{}
        }

        newheaders := make([]string, len(response.headers[name])+1)
        copy(newheaders, response.headers[name])
        newheaders[len(newheaders)-1] = value
        response.headers[name] = newheaders

        //if the header is a cookie, set it
        if name == "Set-Cookie" {
            i := strings.Index(value, ";")
            cookie := value[0:i]
            cookieParts := strings.Split(cookie, "=", 2)

            response.cookies[strings.TrimSpace(cookieParts[0])] = strings.TrimSpace(cookieParts[1])
        }
    }

    return &response
}

func getTestResponse(method string, path string, body string, headers map[string]string) *testResponse {

    req := buildTestRequest(method, path, body, headers)
    var buf bytes.Buffer

    tcpb := tcpBuffer{nil, &buf}
    c := scgiConn{wroteHeaders: false, headers: make(map[string][]string), fd: &tcpb}
    routeHandler(req, &c)
    return buildTestResponse(&buf)

}

type Test struct {
    method         string
    path           string
    body           string
    expectedStatus int
    expectedBody   string
}


//initialize the routes
func init() {
    Get("/", func() string { return "index" })
    Get("/echo/(.*)", func(s string) string { return s })
    Get("/multiecho/(.*)/(.*)/(.*)/(.*)", func(a, b, c, d string) string { return a + b + c + d })
    Post("/post/echo/(.*)", func(s string) string { return s })
    Post("/post/echoparam/(.*)", func(ctx *Context, name string) string { return ctx.Request.Params[name][0] })

    Get("/error/code/(.*)", func(ctx *Context, code string) string {
        n, _ := strconv.Atoi(code)
        message := statusText[n]
        ctx.Abort(n, message)
        return ""
    })

    Get("/error/notfound/(.*)", func(ctx *Context, message string) { ctx.NotFound(message) })

    Post("/posterror/code/(.*)/(.*)", func(ctx *Context, code string, message string) string {
        n, _ := strconv.Atoi(code)
        ctx.Abort(n, message)
        return ""
    })

    Get("/writetest", func(ctx *Context) { ctx.WriteString("hello") })

    Post("/securecookie/set/(.+)/(.+)", func(ctx *Context, name string, val string) string {
        ctx.SetSecureCookie(name, val, 60)
        return ""
    })

    Get("/securecookie/get/(.+)", func(ctx *Context, name string) string {
        val, ok := ctx.GetSecureCookie(name)
        if !ok {
            return ""
        }
        return val
    })
}

var tests = []Test{
    Test{"GET", "/", "", 200, "index"},
    Test{"GET", "/echo/hello", "", 200, "hello"},
    Test{"GET", "/multiecho/a/b/c/d", "", 200, "abcd"},
    Test{"POST", "/post/echo/hello", "", 200, "hello"},
    Test{"POST", "/post/echoparam/a", "a=hello", 200, "hello"},
    //long url
    Test{"GET", "/echo/" + strings.Repeat("0123456789", 100), "", 200, strings.Repeat("0123456789", 100)},

    Test{"GET", "/writetest", "", 200, "hello"},
    Test{"GET", "/error/notfound/notfound", "", 404, "notfound"},
    Test{"GET", "/doesnotexist", "", 404, "Page not found"},
    Test{"POST", "/doesnotexist", "", 404, "Page not found"},
    Test{"GET", "/error/code/500", "", 500, statusText[500]},
    Test{"POST", "/posterror/code/410/failedrequest", "", 410, "failedrequest"},
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
        resp := getTestResponse(test.method, test.path, test.body, make(map[string]string))
        if resp.statusCode != test.expectedStatus {
            t.Fatalf("expected status %d got %d", test.expectedStatus, resp.statusCode)
        }
        if resp.body != test.expectedBody {
            t.Fatalf("expected %q got %q", test.expectedBody, resp.body)
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
        nb := tcpBuffer{input: req, output: &output}
        handleScgiRequest(&nb)
        resp := buildTestResponse(&output)

        if resp.statusCode != test.expectedStatus {
            t.Fatalf("expected status %d got %d", test.expectedStatus, resp.statusCode)
        }

        if resp.body != test.expectedBody {
            t.Fatalf("Scgi expected %q got %q", test.expectedBody, resp.body)
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
        nb := tcpBuffer{input: req, output: &output}
        handleFcgiConnection(&nb)
        contents := getFcgiOutput(&output)
        resp := buildTestResponse(contents)

        if resp.statusCode != test.expectedStatus {
            t.Fatalf("expected status %d got %d", test.expectedStatus, resp.statusCode)
        }

        if resp.body != test.expectedBody {
            t.Fatalf("Fcgi exected %q got %q", test.expectedBody, resp.body)
        }
    }
}


func TestFcgiChucks(t *testing.T) {
    //split up an fcgi request
    bodychunks := []string{`a=12&b=`, strings.Repeat("1234567890", 200)}

    req := buildTestFcgiRequest("POST", "/post/echoparam/b", bodychunks, make(map[string]string))
    var output bytes.Buffer
    nb := tcpBuffer{input: req, output: &output}
    handleFcgiConnection(&nb)
    contents := getFcgiOutput(&output)
    resp := buildTestResponse(contents)

    if resp.body != strings.Repeat("1234567890", 200) {
        t.Fatalf("Fcgi chunks test failed")
    }
}

func TestSecureCookie(t *testing.T) {
    SetCookieSecret("7C19QRmwf3mHZ9CPAaPQ0hsWeufKd")
    resp1 := getTestResponse("POST", "/securecookie/set/a/1", "", nil)
    sval, ok := resp1.cookies["a"]
    if !ok {
        t.Fatalf("Failed to get cookie ")
    }
    cookie := fmt.Sprintf("%s=%s", "a", sval)
    resp2 := getTestResponse("GET", "/securecookie/get/a", "", map[string]string{"Cookie": cookie})

    if resp2.body != "1" {
        t.Fatalf("SecureCookie test failed")
    }
}
