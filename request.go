package web

import (
    "bytes"
    "container/vector"
    "fmt"
    "http"
    "io"
    "io/ioutil"
    "os"
    "strings"
)

type filedata struct {
    Filename string
    Data     []byte
}

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
    Files      map[string]filedata
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
func (r *Request) parseParams() (err os.Error) {
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
        case "multipart/form-data":
            r.Files = make(map[string]filedata)
            boundary := strings.Split(ct, "boundary=", 2)[1]
            var b []byte
            if b, err = ioutil.ReadAll(r.Body); err != nil {
                return err
            }
            parts := bytes.Split(b, strings.Bytes("--"+boundary+"--\r\n"), 0)
            parts = bytes.Split(parts[0], strings.Bytes("--"+boundary+"\r\n"), 0)
            for _, data := range (parts) {
                if len(data) < 2 {
                    continue
                }
                data = data[0 : len(data)-2] // remove the \r\n
                var line []byte
                var rest = data
                //content-disposition params
                cdparams := map[string]string{}
                for {
                    res := bytes.Split(rest, []byte{'\r', '\n'}, 2)
                    if len(res) != 2 {
                        break
                    }
                    line = res[0]
                    rest = res[1]
                    if len(line) == 0 {
                        break
                    }

                    header := strings.Split(string(line), ":", 2)
                    n := strings.TrimSpace(header[0])
                    v := strings.TrimSpace(header[1])
                    if n == "Content-Disposition" {
                        cdparts := strings.Split(v, ";", 0)
                        for _, cdparam := range (cdparts[1:]) {
                            split := strings.Split(cdparam, "=", 2)
                            pname := strings.TrimSpace(split[0])
                            pval := strings.TrimSpace(split[1])
                            cdparams[pname] = pval
                        }
                    }
                }
                //if the param doesn't have a name, ignore it
                if _, ok := cdparams["name"]; !ok {
                    continue
                }
                name := cdparams["name"]
                //check if name is quoted
                if strings.HasPrefix(name, `"`) {
                    name = name[1 : len(name)-1]
                }

                //if it's a file, store it in the Files member
                if filename, ok := cdparams["filename"]; ok {
                    if strings.HasPrefix(filename, `"`) {
                        filename = filename[1 : len(filename)-1]
                    }
                    r.Files[name] = filedata{filename, rest}
                } else {
                    _, ok := r.Params[name]
                    if !ok {
                        r.Params[name] = []string{}
                    }
                    curlen := len(r.Params[name])
                    newlst := make([]string, curlen+1)
                    copy(newlst, r.Params[name])
                    newlst[curlen] = string(rest)
                    r.Params[name] = newlst
                }
            }
        default:
            return &badStringError{"unknown Content-Type", ct}
        }
    }
    return parseForm(r.Params, query)
}

func (r *Request) parseCookies() (err os.Error) {
    if r.Cookies != nil {
        return
    }

    r.Cookies = make(map[string]string)

    for k, v := range (r.Headers) {
        if k == "Cookie" {
            cookies := strings.Split(v, ";", 0)
            for _, cookie := range (cookies) {
                cookie = strings.TrimSpace(cookie)
                parts := strings.Split(cookie, "=", 2)
                if len(parts) != 2 {
                    continue
                }
                r.Cookies[parts[0]] = parts[1]
            }
        }
    }

    return nil
}

func (r *Request) HasParam(name string) bool {
    if r.Params == nil || len(r.Params) == 0 {
        return false
    }
    _, ok := r.Params[name]
    return ok
}
func (r *Request) HasFile(name string) bool {
    if r.Files == nil || len(r.Files) == 0 {
        return false
    }
    _, ok := r.Files[name]
    return ok
}
