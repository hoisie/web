package web

import (
    "container/vector"
    "fmt"
    "http"
    "io"
    "io/ioutil"
    "mime"
    "mime/multipart"
    "net"
    "os"
    "reflect"
    "strconv"
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
    FullParams map[string][]string
    Params     map[string]string
    Cookies    map[string]string
    Files      map[string]filedata
    RemoteAddr string
    RemotePort int
}


type badStringError struct {
    what string
    str  string
}

func (e *badStringError) String() string { return fmt.Sprintf("%s %q", e.what, e.str) }

func flattenParams(fullParams map[string][]string) map[string]string {
    params := map[string]string{}
    for name, lst := range fullParams {
        if len(lst) > 0 {
            params[name] = lst[0]
        }
    }
    return params
}

func newRequest(hr *http.Request, hc http.ResponseWriter) *Request {

    remoteAddr, _ := net.ResolveTCPAddr(hc.RemoteAddr())

    req := Request{
        Method:     hr.Method,
        RawURL:     hr.RawURL,
        URL:        hr.URL,
        Proto:      hr.Proto,
        ProtoMajor: hr.ProtoMajor,
        ProtoMinor: hr.ProtoMinor,
        Headers:    hr.Header,
        Body:       hr.Body,
        Close:      hr.Close,
        Host:       hr.Host,
        Referer:    hr.Referer,
        UserAgent:  hr.UserAgent,
        FullParams: hr.Form,
        RemoteAddr: remoteAddr.IP.String(),
        RemotePort: remoteAddr.Port,
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
    remoteAddr, _ := headers["REMOTE_ADDR"]
    remotePort, _ := strconv.Atoi(headers["REMOTE_PORT"])

    if cookie, ok := headers["HTTP_COOKIE"]; ok {
        httpheader["Cookie"] = cookie
    }

    if method == "POST" {
        if ctype, ok := headers["CONTENT_TYPE"]; ok {
            httpheader["Content-Type"] = ctype
        }

        if clength, ok := headers["CONTENT_LENGTH"]; ok {
            httpheader["Content-Length"] = clength
        }
    }

    req := Request{
        Method:     method,
        RawURL:     rawurl,
        URL:        url,
        Proto:      proto,
        Host:       host,
        UserAgent:  useragent,
        Body:       body,
        Headers:    httpheader,
        RemoteAddr: remoteAddr,
        RemotePort: remotePort,
    }

    return &req
}

func parseForm(m map[string][]string, query string) (err os.Error) {
    data := make(map[string]*vector.StringVector)
    for _, kv := range strings.Split(query, "&", -1) {
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
        m[k] = vec.Copy()
    }

    return
}

// ParseForm parses the request body as a form for POST requests, or the raw query for GET requests.
// It is idempotent.
func (r *Request) parseParams() (err os.Error) {
    if r.Params != nil {
        return
    }
    r.FullParams = make(map[string][]string)
    var query string
    switch r.Method {
    case "GET", "HEAD":
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
            _, params := mime.ParseMediaType(ct)
            boundary, ok := params["boundary"]
            if !ok {
                return os.NewError("Missing Boundary")
            }
            reader := multipart.NewReader(r.Body, boundary)
            r.Files = make(map[string]filedata)
            for {
                part, err := reader.NextPart()
                if err != nil {
                    return err
                }

                if part == nil {
                    break
                }
                //read the data
                data, _ := ioutil.ReadAll(part)
                //check for the 'filename' param
                v, ok := part.Header["Content-Disposition"]
                if !ok {
                    continue
                }
                name := part.FormName()
                d, params := mime.ParseMediaType(v)
                if d != "form-data" {
                    continue
                }
                if params["filename"] != "" {
                    r.Files[name] = filedata{name, data}
                } else {
                    var params vector.StringVector = r.FullParams[name]
                    params.Push(string(data))
                    r.FullParams[name] = params
                }

            }
        default:
            return &badStringError{"unknown Content-Type", ct}
        }
    }
    err = parseForm(r.FullParams, query)
    if err != nil {
        return err
    }
    r.Params = flattenParams(r.FullParams)
    return nil
}

func (r *Request) parseCookies() (err os.Error) {
    if r.Cookies != nil {
        return
    }

    r.Cookies = make(map[string]string)

    if v, ok := r.Headers["Cookie"]; ok {
        cookies := strings.Split(v, ";", -1)
        for _, cookie := range cookies {
            cookie = strings.TrimSpace(cookie)
            parts := strings.Split(cookie, "=", 2)
            if len(parts) != 2 {
                continue
            }
            r.Cookies[parts[0]] = parts[1]
        }
    }

    return nil
}

func (r *Request) HasFile(name string) bool {
    if r.Files == nil || len(r.Files) == 0 {
        return false
    }
    _, ok := r.Files[name]
    return ok
}

func writeTo(s string, val reflect.Value) os.Error {
    switch v := val.(type) {
    // if we're writing to an interace value, just set the byte data
    // TODO: should we support writing to a pointer?
    case *reflect.InterfaceValue:
        v.Set(reflect.NewValue(s))
    case *reflect.BoolValue:
        v.Set(true)
    case *reflect.IntValue:
        i, err := strconv.Atoi64(s)
        if err != nil {
            return err
        }
        v.Set(i)
    case *reflect.UintValue:
        ui, err := strconv.Atoui64(s)
        if err != nil {
            return err
        }
        v.Set(ui)
    case *reflect.FloatValue:
        f, err := strconv.Atof64(s)
        if err != nil {
            return err
        }
        v.Set(f)

    case *reflect.StringValue:
        v.Set(s)
    case *reflect.SliceValue:
        typ := v.Type().(*reflect.SliceType)
        if _, ok := typ.Elem().(*reflect.UintType); ok {
            v.Set(reflect.NewValue([]byte(s)).(*reflect.SliceValue))
        }
    }
    return nil
}

// matchName returns true if key should be written to a field named name.
func matchName(key, name string) bool {
    return strings.ToLower(key) == strings.ToLower(name)
}

func (r *Request) writeToContainer(val reflect.Value) os.Error {
    switch v := val.(type) {
    case *reflect.PtrValue:
        return r.writeToContainer(reflect.Indirect(v))
    case *reflect.InterfaceValue:
        return r.writeToContainer(v.Elem())
    case *reflect.MapValue:
        if _, ok := v.Type().(*reflect.MapType).Key().(*reflect.StringType); !ok {
            return os.NewError("Invalid map type")
        }
        elemtype := v.Type().(*reflect.MapType).Elem()
        for pk, pv := range r.Params {
            mk := reflect.NewValue(pk)
            mv := reflect.MakeZero(elemtype)
            writeTo(pv, mv)
            v.SetElem(mk, mv)
        }
    case *reflect.StructValue:
        for pk, pv := range r.Params {
            //try case sensitive match
            field := v.FieldByName(pk)
            if field != nil {
                writeTo(pv, field)
            }

            //try case insensitive matching
            field = v.FieldByNameFunc(func(s string) bool { return matchName(pk, s) })
            if field != nil {
                writeTo(pv, field)
            }

        }
    default:
        return os.NewError("Invalid container type")
    }
    return nil
}


func (r *Request) UnmarshalParams(val interface{}) os.Error {
    err := r.writeToContainer(reflect.NewValue(val))
    if err != nil {
        return err
    }

    return nil
}
