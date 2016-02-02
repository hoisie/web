package web

import (
    "bytes"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "runtime"
    "strconv"
    "strings"
    "testing"
)

func init() {
    runtime.GOMAXPROCS(4)
}

// ioBuffer is a helper that implements io.ReadWriteCloser,
// which is helpful in imitating a net.Conn
type ioBuffer struct {
    input  *bytes.Buffer
    output *bytes.Buffer
    closed bool
}

func (buf *ioBuffer) Write(p []uint8) (n int, err error) {
    if buf.closed {
        return 0, errors.New("Write after Close on ioBuffer")
    }
    return buf.output.Write(p)
}

func (buf *ioBuffer) Read(p []byte) (n int, err error) {
    if buf.closed {
        return 0, errors.New("Read after Close on ioBuffer")
    }
    return buf.input.Read(p)
}

//noop
func (buf *ioBuffer) Close() error {
    buf.closed = true
    return nil
}

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
    contents := strings.SplitN(s, "\r\n\r\n", 2)

    header := contents[0]

    if len(contents) > 1 {
        response.body = contents[1]
    }

    headers := strings.Split(header, "\r\n")

    statusParts := strings.SplitN(headers[0], " ", 3)
    response.statusCode, _ = strconv.Atoi(statusParts[1])

    for _, h := range headers[1:] {
        split := strings.SplitN(h, ":", 2)
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
            cookieParts := strings.SplitN(cookie, "=", 2)
            response.cookies[strings.TrimSpace(cookieParts[0])] = strings.TrimSpace(cookieParts[1])
        }
    }

    return &response
}

func getTestResponse(method string, path string, body string, headers map[string][]string, cookies []*http.Cookie) *testResponse {
    req := buildTestRequest(method, path, body, headers, cookies)
    var buf bytes.Buffer

    tcpb := ioBuffer{input: nil, output: &buf}
    c := scgiConn{wroteHeaders: false, req: req, headers: make(map[string][]string), fd: &tcpb}
    mainServer.Process(&c, req)
    return buildTestResponse(&buf)
}

func testGet(path string, headers map[string]string) *testResponse {
    var header http.Header
    for k, v := range headers {
        header.Set(k, v)
    }
    return getTestResponse("GET", path, "", header, nil)
}

type Test struct {
    method         string
    path           string
    headers        map[string][]string
    body           string
    expectedStatus int
    expectedBody   string
}

//initialize the routes
func init() {
    mainServer.SetLogger(log.New(ioutil.Discard, "", 0))

    Middleware(func(ctx *Context) string {
        ctx.SetHeader("X-Middleware-Hit", "true", true)
        return ""
    })

    Get("/", func() string { return "index" })
    Get("/panic", func() { panic(0) })
    Get("/echo/(.*)", func(s string) string { return s })
    Get("/multiecho/(.*)/(.*)/(.*)/(.*)", func(a, b, c, d string) string { return a + b + c + d })
    Post("/post/echo/(.*)", func(s string) string { return s })
    Post("/post/echoparam/(.*)", func(ctx *Context, name string) string { return ctx.Params[name] })

    Get("/error/code/(.*)", func(ctx *Context, code string) string {
        n, _ := strconv.Atoi(code)
        message := statusText[n]
        ctx.Abort(n, message)
        return ""
    })

    Get("/error/notfound/(.*)", func(ctx *Context, message string) { ctx.NotFound(message) })

    Get("/error/unauthorized", func(ctx *Context) { ctx.Unauthorized() })
    Post("/error/unauthorized", func(ctx *Context) { ctx.Unauthorized() })

    Get("/error/forbidden", func(ctx *Context) { ctx.Forbidden() })
    Post("/error/forbidden", func(ctx *Context) { ctx.Forbidden() })

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
    Get("/getparam", func(ctx *Context) string { return ctx.Params["a"] })
    Get("/fullparams", func(ctx *Context) string {
        return strings.Join(ctx.Request.Form["a"], ",")
    })

    Get("/json", func(ctx *Context) string {
        ctx.ContentType("json")
        data, _ := json.Marshal(ctx.Params)
        return string(data)
    })

    Get("/jsonbytes", func(ctx *Context) []byte {
        ctx.ContentType("json")
        data, _ := json.Marshal(ctx.Params)
        return data
    })

    Post("/parsejson", func(ctx *Context) string {
        var tmp = struct {
            A   string
            B   string
        }{}
        json.NewDecoder(ctx.Request.Body).Decode(&tmp)
        return tmp.A + " " + tmp.B
    })

    Match("OPTIONS", "/options", func(ctx *Context) {
        ctx.SetHeader("Access-Control-Allow-Methods", "POST, GET, OPTIONS", true)
        ctx.SetHeader("Access-Control-Max-Age", "1000", true)
        ctx.WriteHeader(200)
    })

    Get("/dupeheader", func(ctx *Context) string {
        ctx.SetHeader("Server", "myserver", true)
        return ""
    })

    Get("/authorization", func(ctx *Context) string {
        user, pass, err := ctx.GetBasicAuth()
        if err != nil {
            return "fail"
        }
        return user + pass
    })

    // Test for multiple functions in a call. f1, f2 and f3 should be hit, f4
    // should be skipped since f3 finishes. Thus the overall response
    // should be an HTTP 401 (unauthorized) with a body "response1\nresponse3"
    f1 := func(ctx *Context) string {
        ctx.Unauthorized()
        return "response1\n"
    }

    f2 := func(ctx *Context) []string {
        return nil
    }

    f3 := func(ctx *Context) string {
        ctx.Finish()
        return "response3"
    }

    f4 := func(ctx *Context) string {
        return "response4"
    }

    Get("/multifunc", f1, f2, f3, f4)

    // Test for no handler specification. Should return a 404.
    Get("/nofunc")    
}

var tests = []Test{
    {"GET", "/", nil, "", 200, "index"},
    {"GET", "/echo/hello", nil, "", 200, "hello"},
    {"GET", "/echo/hello", nil, "", 200, "hello"},
    {"GET", "/multiecho/a/b/c/d", nil, "", 200, "abcd"},
    {"POST", "/post/echo/hello", nil, "", 200, "hello"},
    {"POST", "/post/echo/hello", nil, "", 200, "hello"},
    {"POST", "/post/echoparam/a", map[string][]string{"Content-Type": {"application/x-www-form-urlencoded"}}, "a=hello", 200, "hello"},
    {"POST", "/post/echoparam/c?c=hello", nil, "", 200, "hello"},
    {"POST", "/post/echoparam/a", map[string][]string{"Content-Type": {"application/x-www-form-urlencoded"}}, "a=hello\x00", 200, "hello\x00"},
    //long url
    {"GET", "/echo/" + strings.Repeat("0123456789", 100), nil, "", 200, strings.Repeat("0123456789", 100)},
    {"GET", "/writetest", nil, "", 200, "hello"},
    {"GET", "/error/unauthorized", nil, "", 401, ""},
    {"POST", "/error/unauthorized", nil, "", 401, ""},
    {"GET", "/error/forbidden", nil, "", 403, ""},
    {"POST", "/error/forbidden", nil, "", 403, ""},
    {"GET", "/error/notfound/notfound", nil, "", 404, "notfound"},
    {"GET", "/doesnotexist", nil, "", 404, "Page not found"},
    {"POST", "/doesnotexist", nil, "", 404, "Page not found"},
    {"GET", "/error/code/500", nil, "", 500, statusText[500]},
    {"POST", "/posterror/code/410/failedrequest", nil, "", 410, "failedrequest"},
    {"GET", "/getparam?a=abcd", nil, "", 200, "abcd"},
    {"GET", "/getparam?b=abcd", nil, "", 200, ""},
    {"GET", "/fullparams?a=1&a=2&a=3", nil, "", 200, "1,2,3"},
    {"GET", "/panic", nil, "", 500, "Server Error"},
    {"GET", "/json?a=1&b=2", nil, "", 200, `{"a":"1","b":"2"}`},
    {"GET", "/jsonbytes?a=1&b=2", nil, "", 200, `{"a":"1","b":"2"}`},
    {"POST", "/parsejson", map[string][]string{"Content-Type": {"application/json"}}, `{"a":"hello", "b":"world"}`, 200, "hello world"},
    //{"GET", "/testenv", "", 200, "hello world"},
    {"GET", "/authorization", map[string][]string{"Authorization": {BuildBasicAuthCredentials("foo", "bar")}}, "", 200, "foobar"},
    {"GET", "/multifunc", nil, "", 401, "response1\nresponse3"},
    {"GET", "/nofunc", nil, "", 404, "Page not found"},
}

func buildTestRequest(method string, path string, body string, headers map[string][]string, cookies []*http.Cookie) *http.Request {
    host := "127.0.0.1"
    port := "80"
    rawurl := "http://" + host + ":" + port + path
    url_, _ := url.Parse(rawurl)
    proto := "HTTP/1.1"

    if headers == nil {
        headers = map[string][]string{}
    }

    headers["User-Agent"] = []string{"web.go test"}
    if method == "POST" {
        headers["Content-Length"] = []string{fmt.Sprintf("%d", len(body))}
        if headers["Content-Type"] == nil {
            headers["Content-Type"] = []string{"text/plain"}
        }
    }

    req := http.Request{Method: method,
        URL:    url_,
        Proto:  proto,
        Host:   host,
        Header: http.Header(headers),
        Body:   ioutil.NopCloser(bytes.NewBufferString(body)),
    }

    for _, cookie := range cookies {
        req.AddCookie(cookie)
    }
    return &req
}

func TestRouting(t *testing.T) {
    for _, test := range tests {
        resp := getTestResponse(test.method, test.path, test.body, test.headers, nil)

        if resp.statusCode != test.expectedStatus {
            t.Fatalf("%v(%v) expected status %d got %d", test.method, test.path, test.expectedStatus, resp.statusCode)
        }
        
        if resp.body != test.expectedBody {
            t.Fatalf("%v(%v) expected %q got %q", test.method, test.path, test.expectedBody, resp.body)
        }
        
        if cl, ok := resp.headers["Content-Length"]; ok {
            clExp, _ := strconv.Atoi(cl[0])
            clAct := len(resp.body)
            if clExp != clAct {
                t.Fatalf("Content-length doesn't match. expected %d got %d", clExp, clAct)
            }
        }

        mwh, ok := resp.headers["X-Middleware-Hit"]

        if !ok {
            t.Fatalf("%v(%v) middleware hit header not set", test.method, test.path)
        } else if mwh[0] != "true" {
            t.Fatalf("%v(%v) middleware hit header incorrectly set", test.method, test.path)
        }
    }
}

func TestHead(t *testing.T) {
    for _, test := range tests {

        if test.method != "GET" {
            continue
        }
        getresp := getTestResponse("GET", test.path, test.body, test.headers, nil)
        headresp := getTestResponse("HEAD", test.path, test.body, test.headers, nil)

        if getresp.statusCode != headresp.statusCode {
            t.Fatalf("head and get status differ. expected %d got %d", getresp.statusCode, headresp.statusCode)
        }
        if len(headresp.body) != 0 {
            t.Fatalf("head request arrived with a body")
        }

        var cl []string
        var getcl, headcl int
        var hascl1, hascl2 bool

        if cl, hascl1 = getresp.headers["Content-Length"]; hascl1 {
            getcl, _ = strconv.Atoi(cl[0])
        }

        if cl, hascl2 = headresp.headers["Content-Length"]; hascl2 {
            headcl, _ = strconv.Atoi(cl[0])
        }

        if hascl1 != hascl2 {
            t.Fatalf("head and get: one has content-length, one doesn't")
        }

        if hascl1 == true && getcl != headcl {
            t.Fatalf("head and get content-length differ")
        }
    }
}

func buildTestScgiRequest(method string, path string, body string, headers map[string][]string) *bytes.Buffer {
    var headerBuf bytes.Buffer
    scgiHeaders := make(map[string]string)

    headerBuf.WriteString("CONTENT_LENGTH")
    headerBuf.WriteByte(0)
    headerBuf.WriteString(fmt.Sprintf("%d", len(body)))
    headerBuf.WriteByte(0)

    scgiHeaders["REQUEST_METHOD"] = method
    scgiHeaders["HTTP_HOST"] = "127.0.0.1"
    scgiHeaders["REQUEST_URI"] = path
    scgiHeaders["SERVER_PORT"] = "80"
    scgiHeaders["SERVER_PROTOCOL"] = "HTTP/1.1"
    scgiHeaders["USER_AGENT"] = "web.go test framework"

    for k, v := range headers {
        //Skip content-length
        if k == "Content-Length" {
            continue
        }
        key := "HTTP_" + strings.ToUpper(strings.Replace(k, "-", "_", -1))
        scgiHeaders[key] = v[0]
    }
    for k, v := range scgiHeaders {
        headerBuf.WriteString(k)
        headerBuf.WriteByte(0)
        headerBuf.WriteString(v)
        headerBuf.WriteByte(0)
    }
    headerData := headerBuf.Bytes()

    var buf bytes.Buffer
    //extra 1 is for the comma at the end
    dlen := len(headerData)
    fmt.Fprintf(&buf, "%d:", dlen)
    buf.Write(headerData)
    buf.WriteByte(',')
    buf.WriteString(body)
    return &buf
}

func TestScgi(t *testing.T) {
    for _, test := range tests {
        req := buildTestScgiRequest(test.method, test.path, test.body, test.headers)
        var output bytes.Buffer
        nb := ioBuffer{input: req, output: &output}
        mainServer.handleScgiRequest(&nb)
        resp := buildTestResponse(&output)

        if resp.statusCode != test.expectedStatus {
            t.Fatalf("expected status %d got %d", test.expectedStatus, resp.statusCode)
        }

        if resp.body != test.expectedBody {
            t.Fatalf("Scgi expected %q got %q", test.expectedBody, resp.body)
        }
    }
}

func TestScgiHead(t *testing.T) {
    for _, test := range tests {

        if test.method != "GET" {
            continue
        }

        req := buildTestScgiRequest("GET", test.path, test.body, make(map[string][]string))
        var output bytes.Buffer
        nb := ioBuffer{input: req, output: &output}
        mainServer.handleScgiRequest(&nb)
        getresp := buildTestResponse(&output)

        req = buildTestScgiRequest("HEAD", test.path, test.body, make(map[string][]string))
        var output2 bytes.Buffer
        nb = ioBuffer{input: req, output: &output2}
        mainServer.handleScgiRequest(&nb)
        headresp := buildTestResponse(&output2)

        if getresp.statusCode != headresp.statusCode {
            t.Fatalf("head and get status differ. expected %d got %d", getresp.statusCode, headresp.statusCode)
        }
        if len(headresp.body) != 0 {
            t.Fatalf("head request arrived with a body")
        }

        var cl []string
        var getcl, headcl int
        var hascl1, hascl2 bool

        if cl, hascl1 = getresp.headers["Content-Length"]; hascl1 {
            getcl, _ = strconv.Atoi(cl[0])
        }

        if cl, hascl2 = headresp.headers["Content-Length"]; hascl2 {
            headcl, _ = strconv.Atoi(cl[0])
        }

        if hascl1 != hascl2 {
            t.Fatalf("head and get: one has content-length, one doesn't")
        }

        if hascl1 == true && getcl != headcl {
            t.Fatalf("head and get content-length differ")
        }
    }
}

func TestReadScgiRequest(t *testing.T) {
    headers := map[string][]string{"User-Agent": {"web.go"}}
    req := buildTestScgiRequest("POST", "/hello", "Hello world!", headers)
    var s Server
    httpReq, err := s.readScgiRequest(&ioBuffer{input: req, output: nil})
    if err != nil {
        t.Fatalf("Error while reading SCGI request: ", err.Error())
    }
    if httpReq.ContentLength != 12 {
        t.Fatalf("Content length mismatch, expected %d, got %d ", 12, httpReq.ContentLength)
    }
    var body bytes.Buffer
    io.Copy(&body, httpReq.Body)
    if body.String() != "Hello world!" {
        t.Fatalf("Body mismatch, expected %q, got %q ", "Hello world!", body.String())
    }
}

func makeCookie(vals map[string]string) []*http.Cookie {
    var cookies []*http.Cookie
    for k, v := range vals {
        c := &http.Cookie{
            Name:  k,
            Value: v,
        }
        cookies = append(cookies, c)
    }
    return cookies
}

func TestSecureCookie(t *testing.T) {
    mainServer.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"
    resp1 := getTestResponse("POST", "/securecookie/set/a/1", "", nil, nil)
    sval, ok := resp1.cookies["a"]
    if !ok {
        t.Fatalf("Failed to get cookie ")
    }
    cookies := makeCookie(map[string]string{"a": sval})

    resp2 := getTestResponse("GET", "/securecookie/get/a", "", nil, cookies)

    if resp2.body != "1" {
        t.Fatalf("SecureCookie test failed")
    }
}

func TestEarlyClose(t *testing.T) {
    var server1 Server
    server1.Close()
}

func TestOptions(t *testing.T) {
    resp := getTestResponse("OPTIONS", "/options", "", nil, nil)
    if resp.headers["Access-Control-Allow-Methods"][0] != "POST, GET, OPTIONS" {
        t.Fatalf("TestOptions - Access-Control-Allow-Methods failed")
    }
    if resp.headers["Access-Control-Max-Age"][0] != "1000" {
        t.Fatalf("TestOptions - Access-Control-Max-Age failed")
    }
}

func TestSlug(t *testing.T) {
    tests := [][]string{
        {"", ""},
        {"a", "a"},
        {"a/b", "a-b"},
        {"a b", "a-b"},
        {"a////b", "a-b"},
        {" a////b ", "a-b"},
        {" Manowar / Friends ", "manowar-friends"},
    }

    for _, test := range tests {
        v := Slug(test[0], "-")
        if v != test[1] {
            t.Fatalf("TestSlug(%v) failed, expected %v, got %v", test[0], test[1], v)
        }
    }
}

// tests that we don't duplicate headers
func TestDuplicateHeader(t *testing.T) {
    resp := testGet("/dupeheader", nil)
    if len(resp.headers["Server"]) > 1 {
        t.Fatalf("Expected only one header, got %#v", resp.headers["Server"])
    }
    if resp.headers["Server"][0] != "myserver" {
        t.Fatalf("Incorrect header, exp 'myserver', got %q", resp.headers["Server"][0])
    }
}

func BuildBasicAuthCredentials(user string, pass string) string {
    s := user + ":" + pass
    return "Basic " + base64.StdEncoding.EncodeToString([]byte(s))
}

func BenchmarkProcessGet(b *testing.B) {
    s := NewServer()
    s.SetLogger(log.New(ioutil.Discard, "", 0))
    s.Get("/echo/(.*)", func(s string) string {
        return s
    })
    req := buildTestRequest("GET", "/echo/hi", "", nil, nil)
    var buf bytes.Buffer
    iob := ioBuffer{input: nil, output: &buf}
    c := scgiConn{wroteHeaders: false, req: req, headers: make(map[string][]string), fd: &iob}
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        s.Process(&c, req)
    }
}

func BenchmarkProcessPost(b *testing.B) {
    s := NewServer()
    s.SetLogger(log.New(ioutil.Discard, "", 0))
    s.Post("/echo/(.*)", func(s string) string {
        return s
    })
    req := buildTestRequest("POST", "/echo/hi", "", nil, nil)
    var buf bytes.Buffer
    iob := ioBuffer{input: nil, output: &buf}
    c := scgiConn{wroteHeaders: false, req: req, headers: make(map[string][]string), fd: &iob}
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        s.Process(&c, req)
    }
}
