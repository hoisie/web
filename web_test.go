// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var nopLogger = log.New(ioutil.Discard, "", 0)

func init() {
	runtime.GOMAXPROCS(4)
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

func getTestResponse(s *Server, method string, path string, body string, headers map[string][]string, cookies []*http.Cookie) *testResponse {
	req := buildTestRequest(method, path, body, headers, cookies)
	var buf bytes.Buffer

	tcpb := tcpBuffer{nil, &buf}
	c := &scgiConn{wroteHeaders: false, req: req, headers: make(map[string][]string), fd: &tcpb}
	s.ServeHTTP(c, req)
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

// Initialize test routes
func generalTestServer() *Server {
	s := NewServer()
	s.SetLogger(nopLogger)
	s.Get("/", func() string { return "index" })
	s.Get("/panic", func() { panic(0) })
	s.Get("/echo/(.*)", func(s string) string { return s })
	s.Get("/multiecho/(.*)/(.*)/(.*)/(.*)", func(a, b, c, d string) string { return a + b + c + d })
	s.Post("/post/echo/(.*)", func(s string) string { return s })
	s.Post("/post/echoparam/(.*)", func(ctx *Context, name string) string { return ctx.Params[name] })

	s.Get("/error/code/(.*)", func(ctx *Context, code string) string {
		n, _ := strconv.Atoi(code)
		message := http.StatusText(n)
		ctx.Abort(n, message)
		return ""
	})

	s.Get("/error/notfound/(.*)", func(ctx *Context, message string) (string, error) {
		return "", WebError{404, message}
	})

	s.Post("/posterror/code/(.*)/(.*)", func(ctx *Context, code string, message string) string {
		n, _ := strconv.Atoi(code)
		ctx.Abort(n, message)
		return ""
	})

	s.Get("/writetest", func(ctx *Context) (string, error) { return "hello", nil })

	s.Post("/securecookie/set/(.+)/(.+)", func(ctx *Context, name string, val string) string {
		ctx.SetSecureCookie(name, val, 60)
		return ""
	})

	s.Get("/securecookie/get/(.+)", func(ctx *Context, name string) string {
		val, ok := ctx.GetSecureCookie(name)
		if !ok {
			return ""
		}
		return val
	})
	s.Get("/getparam", func(ctx *Context) string { return ctx.Params["a"] })
	s.Get("/fullparams", func(ctx *Context) string {
		return strings.Join(ctx.Request.Form["a"], ",")
	})

	s.Get("/json", func(ctx *Context) string {
		ctx.ContentType("json")
		data, _ := json.Marshal(ctx.Params)
		return string(data)
	})

	s.Get("/jsonbytes", func(ctx *Context) []byte {
		ctx.ContentType("application/json")
		data, _ := json.Marshal(ctx.Params)
		return data
	})

	type tmptype struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	s.Post("/parsejson", func(ctx *Context) (tmptype, error) {
		ctx.ContentType("application/json")
		tmp := tmptype{"hello", "world"}
		//json.NewDecoder(ctx.Request.Body).Decode(&tmp)
		return tmp, nil
	})
	return s
}

var generalTests = []Test{
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
	{"GET", "/error/notfound/notfound", nil, "", 404, "notfound"},
	{"GET", "/doesnotexist", nil, "", 404, "Page not found"},
	{"POST", "/doesnotexist", nil, "", 404, "Page not found"},
	{"GET", "/error/code/500", nil, "", 500, http.StatusText(500)},
	{"POST", "/posterror/code/410/failedrequest", nil, "", 410, "failedrequest"},
	{"GET", "/getparam?a=abcd", nil, "", 200, "abcd"},
	{"GET", "/getparam?b=abcd", nil, "", 200, ""},
	{"GET", "/fullparams?a=1&a=2&a=3", nil, "", 200, "1,2,3"},
	{"GET", "/panic", nil, "", 500, "Server Error"},
	{"GET", "/json?a=1&b=2", nil, "", 200, `{"a":"1","b":"2"}`},
	{"GET", "/jsonbytes?a=1&b=2", nil, "", 200, `{"a":"1","b":"2"}`},
	{"POST", "/parsejson", map[string][]string{"Content-Type": {"application/json"}, "Accept": {"application/json"}}, `{"a":"hello", "b":"world"}`, 200, `{"a":"hello","b":"world"}
`},
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
		headers["Content-Length"] = []string{strconv.Itoa(len(body))}
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

func testRouting(t *testing.T, s *Server, tests []Test) {
	for _, test := range tests {
		resp := getTestResponse(s, test.method, test.path, test.body, test.headers, nil)

		if resp.statusCode != test.expectedStatus {
			t.Fatalf("%v: expected status %d got %d", test.path, test.expectedStatus, resp.statusCode)
		}
		if resp.body != test.expectedBody {
			t.Fatalf("%v: expected %q got %q", test.path, test.expectedBody, resp.body)
		}
		if cl, ok := resp.headers["Content-Length"]; ok {
			clExp, _ := strconv.Atoi(cl[0])
			clAct := len(resp.body)
			if clExp != clAct {
				t.Fatalf("%v: Content-length doesn't match. expected %d got %d", test.path, clExp, clAct)
			}
		}
	}
}

func TestRouting(t *testing.T) {
	s := generalTestServer()
	testRouting(t, s, generalTests)
}

func testHead(t *testing.T, s *Server, tests []Test) {
	for _, test := range tests {

		if test.method != "GET" {
			continue
		}
		getresp := getTestResponse(s, "GET", test.path, test.body, test.headers, nil)
		headresp := getTestResponse(s, "HEAD", test.path, test.body, test.headers, nil)

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

func TestHead(t *testing.T) {
	s := generalTestServer()
	testHead(t, s, generalTests)
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
	s := generalTestServer()
	s.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"
	resp1 := getTestResponse(s, "POST", "/securecookie/set/a/1", "", nil, nil)
	sval, ok := resp1.cookies["a"]
	if !ok {
		t.Fatalf("Failed to get cookie ")
	}
	cookies := makeCookie(map[string]string{"a": sval})

	resp2 := getTestResponse(s, "GET", "/securecookie/get/a", "", nil, cookies)

	if resp2.body != "1" {
		t.Fatalf("SecureCookie test failed")
	}
}

func TestEarlyClose(t *testing.T) {
	var server1 Server
	server1.Close()
}
