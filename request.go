package web

import (
    "container/vector"
    "fmt"
    "http"
    "io"
    "io/ioutil"
    "os"
    "strings"
)

type Request struct {
    Method     string    // GET, POST, PUT, etc.
    RawURL     string    // The raw URL given in the request.
    URL        *http.URL // Parsed URL.
    Proto      string    // "HTTP/1.0"
    ProtoMajor int       // 1
    ProtoMinor int       // 0
    Headers    map[string]string
    Body       io.Reader
    Close      bool
    Host       string
    Referer    string
    UserAgent  string
    Params     map[string][]string
    Cookies    map[string]string
}

type badStringError struct {
    what string
    str  string
}

func (e *badStringError) String() string { return fmt.Sprintf("%s %q", e.what, e.str) }

func newRequest(hr *http.Request) *Request {
    req := Request{
        Method: hr.Method,
        RawURL: hr.RawURL,
        URL: hr.URL,
        Proto: hr.Proto,
        ProtoMajor: hr.ProtoMajor,
        ProtoMinor: hr.ProtoMinor,
        Headers: hr.Header,
        Body: hr.Body,
        Close: hr.Close,
        Host: hr.Host,
        Referer: hr.Referer,
        UserAgent: hr.UserAgent,
        Params: hr.Form,
    }
    return &req
}

func newRequestCgi(headers map[string]string, body io.Reader) *Request {

    var httpheader = make(map[string]string)

    method, _ := headers["REQUEST_METHOD"]
    host, _ := headers["HTTP_HOST"]
    path, _ := headers["REQUEST_URI"]
    port, _ := headers["SERVER_PORT"]
    proto, _ := headers["SERVER_PROTOCOL"]
    rawurl := "http://" + host + ":" + port + path
    url, _ := http.ParseURL(rawurl)
    useragent, _ := headers["USER_AGENT"]

    if method == "POST" {
        if ctype, ok := headers["CONTENT_TYPE"]; ok {
            httpheader["Content-Type"] = ctype
        }

        if clength, ok := headers["CONTENT_LENGTH"]; ok {
            httpheader["Content-Length"] = clength
        }
    }

    req := Request{
        Method: method,
        RawURL: rawurl,
        URL: url,
        Proto: proto,
        Host: host,
        UserAgent: useragent,
        Body: body,
        Headers: httpheader,
    }

    return &req
}

func parseForm(m map[string][]string, query string) (err os.Error) {
    data := make(map[string]*vector.StringVector)
    for _, kv := range strings.Split(query, "&", 0) {
        kvPair := strings.Split(kv, "=", 2)

        var key, value string
        var e os.Error
        key, e = http.URLUnescape(kvPair[0])
        if e == nil && len(kvPair) > 1 {
            value, e = http.URLUnescape(kvPair[1])
        }
        if e != nil {
            err = e
        }

        vec, ok := data[key]
        if !ok {
            vec = new(vector.StringVector)
            data[key] = vec
        }
        vec.Push(value)
    }

    for k, vec := range data {
        m[k] = vec.Data()
    }

    return
}

// ParseForm parses the request body as a form for POST requests, or the raw query for GET requests.
// It is idempotent.
func (r *Request) ParseParams() (err os.Error) {
    if r.Params != nil {
        return
    }
    r.Params = make(map[string][]string)

    var query string
    switch r.Method {
    case "GET":
        query = r.URL.RawQuery
    case "POST":
        if r.Body == nil {
            return os.ErrorString("missing form body")
        }
        ct, _ := r.Headers["Content-Type"]
        switch strings.Split(ct, ";", 2)[0] {
        case "text/plain", "application/x-www-form-urlencoded", "":
            var b []byte
            if b, err = ioutil.ReadAll(r.Body); err != nil {
                return err
            }
            query = string(b)
        // TODO(dsymonds): Handle multipart/form-data
        default:
            return &badStringError{"unknown Content-Type", ct}
        }
    }
    return parseForm(r.Params, query)
}

func (r *Request) ParseCookies() (err os.Error) {
    if r.Cookies != nil {
        return
    }

    r.Cookies = make(map[string]string)

    for k, v := range (r.Headers) {
        if k == "Cookie" {
            cookies := strings.Split(v, ";", 0)
            for _, cookie := range (cookies) {
                cookie = strings.TrimSpace(cookie)
                parts := strings.Split(cookie, "=", 0)
                println("has a cookie", parts[0], parts[1])
                r.Cookies[parts[0]] = parts[1]
            }
        }
    }

    return nil
}
