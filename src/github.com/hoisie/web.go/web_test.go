package web

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	_ "log"
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

func flattenHeaders(headers map[string][]string) map[string]string {
	ret := map[string]string{}
	for k, v := range headers {
		ret[k] = v[0]
	}
	return ret
}

//this implements io.ReadWriteCloser, which means it can be passed around as a tcp connection
type tcpBuffer struct {
	input  *bytes.Buffer
	output *bytes.Buffer
}

func (buf *tcpBuffer) Write(p []uint8) (n int, err error) {
	return buf.output.Write(p)
}

func (buf *tcpBuffer) Read(p []byte) (n int, err error) {
	return buf.input.Read(p)
}

func (buf *tcpBuffer) Close() error { return nil }

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

	tcpb := tcpBuffer{nil, &buf}
	c := scgiConn{wroteHeaders: false, req: req, headers: make(map[string][]string), fd: &tcpb}
	mainServer.routeHandler(req, &c)
	return buildTestResponse(&buf)

}

type Test struct {
	method         string
	path           string
	headers        map[string][]string
	body           string
	expectedStatus int
	expectedBody   string
}

type StructHandler struct {
	a string
}

func (s *StructHandler) method() string {
	return s.a
}

func (s *StructHandler) method2(ctx *Context) string {
	return s.a + ctx.Params["b"]
}

func (s *StructHandler) method3(ctx *Context, b string) string {
	return s.a + b
}

//initialize the routes
func init() {
	//Uncomment the following two lines to see the output of the test
	//f, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0644)
	//mainServer.SetLogger(log.New(f, "", 0))

	//Test the environment
	mainServer.Env["test"] = "hello world"

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
	Get("/getparam", func(ctx *Context) string {
		return ctx.Params["a"]
	})
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
			A string
			B string
		}{}
		json.NewDecoder(ctx.Request.Body).Decode(&tmp)
		return tmp.A + " " + tmp.B
	})

	Get("/testenv", func(ctx *Context) string {
		return ctx.Server.Env["test"].(string)
	})
}

var tests = []Test{
	{"GET", "/", nil, "", 200, "index"},
	{"GET", "/echo/hello", nil, "", 200, "hello"},
	{"GET", "/echo/hello", nil, "", 200, "hello"},
	{"GET", "/multiecho/a/b/c/d", nil, "", 200, "abcd"},
	{"POST", "/post/echo/hello", nil, "", 200, "hello"},
	{"POST", "/post/echo/hello", nil, "", 200, "hello"},
	{"POST", "/post/echoparam/a", nil, "a=hello", 200, "hello"},
	{"POST", "/post/echoparam/c?c=hello", nil, "", 200, "hello"},
	{"POST", "/post/echoparam/a", nil, "a=hello\x00", 200, "hello\x00"},
	//long url
	{"GET", "/echo/" + strings.Repeat("0123456789", 100), nil, "", 200, strings.Repeat("0123456789", 100)},
	{"GET", "/writetest", nil, "", 200, "hello"},
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
		RawURL: rawurl,
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
			t.Fatalf("expected status %d got %d", test.expectedStatus, resp.statusCode)
		}
		if resp.body != test.expectedBody {
			t.Fatalf("expected %q got %q", test.expectedBody, resp.body)
		}
		if cl, ok := resp.headers["Content-Length"]; ok {
			clExp, _ := strconv.Atoi(cl[0])
			clAct := len(resp.body)
			if clExp != clAct {
				t.Fatalf("Content-length doesn't match. expected %d got %d", clExp, clAct)
			}
		}
	}
}

func TestHead(t *testing.T) {
	for _, test := range tests {

		if test.method != "GET" {
			continue
		}
		getresp := getTestResponse("GET", test.path, test.body, make(map[string][]string), nil)
		headresp := getTestResponse("HEAD", test.path, test.body, make(map[string][]string), nil)

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

func buildScgiFields(fields map[string]string, buf *bytes.Buffer) []byte {
	for k, v := range fields {
		buf.WriteString(k)
		buf.WriteByte(0)
		buf.WriteString(v)
		buf.WriteByte(0)
	}

	return buf.Bytes()
}

func buildTestScgiRequest(method string, path string, body string, headers map[string][]string) *bytes.Buffer {
	var hbuf bytes.Buffer
	scgiHeaders := make(map[string]string)

	hbuf.WriteString("CONTENT_LENGTH")
	hbuf.WriteByte(0)
	hbuf.WriteString(fmt.Sprintf("%d", len(body)))
	hbuf.WriteByte(0)

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

	buildScgiFields(scgiHeaders, &hbuf)

	fielddata := hbuf.Bytes()
	var buf bytes.Buffer

	//extra 1 is for the comma at the end
	dlen := len(fielddata) + len(body) + 1
	fmt.Fprintf(&buf, "%d:", dlen)
	buf.Write(fielddata)
	buf.WriteByte(',')
	buf.WriteString(body)

	return &buf
}

func TestScgi(t *testing.T) {
	for _, test := range tests {
		req := buildTestScgiRequest(test.method, test.path, test.body, test.headers)
		var output bytes.Buffer
		nb := tcpBuffer{input: req, output: &output}
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
		nb := tcpBuffer{input: req, output: &output}
		mainServer.handleScgiRequest(&nb)
		getresp := buildTestResponse(&output)

		req = buildTestScgiRequest("HEAD", test.path, test.body, make(map[string][]string))
		var output2 bytes.Buffer
		nb = tcpBuffer{input: req, output: &output2}
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

func buildFcgiKeyValue(key string, val string) []byte {

	var buf bytes.Buffer

	if len(key) <= 127 && len(val) <= 127 {
		data := struct {
			A uint8
			B uint8
		}{uint8(len(key)), uint8(len(val))}
		binary.Write(&buf, binary.BigEndian, data)
	} else if len(key) <= 127 && len(val) > 127 {
		data := struct {
			A uint8
			B uint32
		}{uint8(len(key)), uint32(len(val)) | 1<<31}

		binary.Write(&buf, binary.BigEndian, data)
	}
	buf.WriteString(key)
	buf.WriteString(val)

	return buf.Bytes()
}

func buildTestFcgiRequest(method string, path string, bodychunks []string, headers map[string][]string) *bytes.Buffer {

	var req bytes.Buffer
	fcgiHeaders := make(map[string]string)

	bodylength := 0
	for _, s := range bodychunks {
		bodylength += len(s)
	}

	fcgiHeaders["CONTENT_LENGTH"] = fmt.Sprintf("%d", bodylength)
	fcgiHeaders["REQUEST_METHOD"] = method
	fcgiHeaders["HTTP_HOST"] = "127.0.0.1"
	fcgiHeaders["REQUEST_URI"] = path
	fcgiHeaders["SERVER_PORT"] = "80"
	fcgiHeaders["SERVER_PROTOCOL"] = "HTTP/1.1"
	fcgiHeaders["USER_AGENT"] = "web.go test framework"

	for k, v := range headers {
		//Skip content-length
		if k == "Content-Length" {
			continue
		}
		key := "HTTP_" + strings.ToUpper(strings.Replace(k, "-", "_", -1))
		fcgiHeaders[key] = v[0]
	}
	// add the begin request
	req.Write(newFcgiRecord(fcgiBeginRequest, 0, make([]byte, 8)))

	var buf bytes.Buffer
	for k, v := range fcgiHeaders {
		kv := buildFcgiKeyValue(k, v)
		buf.Write(kv)
	}

	//add the params record
	req.Write(newFcgiRecord(fcgiParams, 0, buf.Bytes()))

	//add the end-of-params record
	req.Write(newFcgiRecord(fcgiParams, 0, []byte{}))

	//send the body
	for _, s := range bodychunks {
		if len(s) > 0 {
			req.Write(newFcgiRecord(fcgiStdin, 0, []byte(s)))
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
		if err == io.EOF {
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
	for _, test := range tests {
		req := buildTestFcgiRequest(test.method, test.path, []string{test.body}, test.headers)
		var output bytes.Buffer
		nb := tcpBuffer{input: req, output: &output}
		mainServer.handleFcgiConnection(&nb)
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

func TestFcgiHead(t *testing.T) {
	for _, test := range tests {

		if test.method != "GET" {
			continue
		}
		req := buildTestFcgiRequest("GET", test.path, []string{test.body}, make(map[string][]string))
		var output bytes.Buffer
		nb := tcpBuffer{input: req, output: &output}
		mainServer.handleFcgiConnection(&nb)
		contents := getFcgiOutput(&output)
		getresp := buildTestResponse(contents)

		req = buildTestFcgiRequest("HEAD", test.path, []string{test.body}, make(map[string][]string))
		var output2 bytes.Buffer
		nb = tcpBuffer{input: req, output: &output2}
		mainServer.handleFcgiConnection(&nb)
		contents = getFcgiOutput(&output2)
		headresp := buildTestResponse(contents)

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

func TestFcgiChunks(t *testing.T) {
	//split up an fcgi request
	bodychunks := []string{`a=12&b=`, strings.Repeat("1234567890", 200)}

	req := buildTestFcgiRequest("POST", "/post/echoparam/b", bodychunks, make(map[string][]string))
	var output bytes.Buffer
	nb := tcpBuffer{input: req, output: &output}
	mainServer.handleFcgiConnection(&nb)
	contents := getFcgiOutput(&output)
	resp := buildTestResponse(contents)

	if resp.body != strings.Repeat("1234567890", 200) {
		t.Fatalf("Fcgi chunks test failed")
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

func TestSecureCookieFcgi(t *testing.T) {
	mainServer.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"

	//set the cookie
	req := buildTestFcgiRequest("POST", "/securecookie/set/a/1", []string{}, make(map[string][]string))

	var output bytes.Buffer
	nb := tcpBuffer{input: req, output: &output}
	mainServer.handleFcgiConnection(&nb)
	contents := getFcgiOutput(&output)
	resp := buildTestResponse(contents)
	sval, ok := resp.cookies["a"]
	if !ok {
		t.Fatalf("SecureCookieFcgi test failed")
	}

	//send the cookie
	cookie := fmt.Sprintf("a=%s", sval)
	req = buildTestFcgiRequest("GET", "/securecookie/get/a", []string{}, map[string][]string{"Cookie": {cookie}})
	var output2 bytes.Buffer
	nb = tcpBuffer{input: req, output: &output2}
	mainServer.handleFcgiConnection(&nb)
	contents = getFcgiOutput(&output2)
	resp = buildTestResponse(contents)

	if resp.body != "1" {
		t.Fatalf("SecureCookie test failed body")
	}
}

func TestEarlyClose(t *testing.T) {
	var server1 Server
	server1.Close()
}
