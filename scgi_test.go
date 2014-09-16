// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

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

func testScgi(t *testing.T, s *Server, test Test) {
	req := buildTestScgiRequest(test.method, test.path, test.body, test.headers)
	var output bytes.Buffer
	nb := tcpBuffer{input: req, output: &output}
	s.handleScgiRequest(&nb)
	resp := buildTestResponse(&output)

	if resp.statusCode != test.expectedStatus {
		t.Fatalf("expected status %d got %d", test.expectedStatus, resp.statusCode)
	}

	if resp.body != test.expectedBody {
		t.Fatalf("Scgi expected %q got %q", test.expectedBody, resp.body)
	}
}

func TestScgi(t *testing.T) {
	s := generalTestServer()
	for _, test := range generalTests {
		testScgi(t, s, test)
	}
}

func testScgiHead(t *testing.T, s *Server, test Test) {
	if test.method != "GET" {
		return
	}

	req := buildTestScgiRequest("GET", test.path, test.body, make(map[string][]string))
	var output bytes.Buffer
	nb := tcpBuffer{input: req, output: &output}
	s.handleScgiRequest(&nb)
	getresp := buildTestResponse(&output)

	req = buildTestScgiRequest("HEAD", test.path, test.body, make(map[string][]string))
	var output2 bytes.Buffer
	nb = tcpBuffer{input: req, output: &output2}
	s.handleScgiRequest(&nb)
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

func TestScgiHead(t *testing.T) {
	s := generalTestServer()
	for _, test := range generalTests {
		testScgiHead(t, s, test)
	}
}
