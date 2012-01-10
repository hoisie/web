package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type filedata struct {
	Filename string
	Data     []byte
}

type Request struct {
	Method     string   // GET, POST, PUT, etc.
	RawURL     string   // The raw URL given in the request.
	URL        *url.URL // Parsed URL.
	Proto      string   // "HTTP/1.0"
	ProtoMajor int      // 1
	ProtoMinor int      // 0
	Headers    http.Header
	Body       io.Reader
	Close      bool
	Host       string
	Referer    string
	UserAgent  string
	FullParams map[string][]string
	Params     map[string]string
	ParamData  []byte
	Cookies    map[string]string
	Cookie     []*http.Cookie
	Files      map[string]filedata
	RemoteAddr string
	RemotePort int
}

type badStringError struct {
	what string
	str  string
}

func (e *badStringError) Error() string { return fmt.Sprintf("%s %q", e.what, e.str) }

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

	remoteAddr, _ := net.ResolveTCPAddr("tcp", hr.RemoteAddr)

	req := Request{
		Method:     hr.Method,
		URL:        hr.URL,
		Proto:      hr.Proto,
		ProtoMajor: hr.ProtoMajor,
		ProtoMinor: hr.ProtoMinor,
		Headers:    hr.Header,
		Body:       hr.Body,
		Close:      hr.Close,
		Host:       hr.Host,
		Referer:    hr.Referer(),
		UserAgent:  hr.UserAgent(),
		FullParams: hr.Form,
		Cookie:     hr.Cookies(),
		RemoteAddr: remoteAddr.IP.String(),
		RemotePort: remoteAddr.Port,
	}
	return &req
}

func newRequestCgi(headers http.Header, body io.Reader) *Request {
	var httpheader = make(http.Header)
	for header, value := range headers {
		if strings.HasPrefix(header, "Http_") {
			newHeader := header[5:]
			newHeader = strings.Replace(newHeader, "_", "-", -1)
			newHeader = http.CanonicalHeaderKey(newHeader)
			httpheader[newHeader] = value
		}
	}

	host := httpheader.Get("Host")
	method := headers.Get("REQUEST_METHOD")
	path := headers.Get("REQUEST_URI")
	port := headers.Get("SERVER_PORT")
	proto := headers.Get("SERVER_PROTOCOL")
	rawurl := "http://" + host + ":" + port + path
	url_, _ := url.Parse(rawurl)
	useragent := headers.Get("USER_AGENT")
	remoteAddr := headers.Get("REMOTE_ADDR")
	remotePort, _ := strconv.Atoi(headers.Get("REMOTE_PORT"))

	if method == "POST" {
		if ctype, ok := headers["CONTENT_TYPE"]; ok {
			httpheader["Content-Type"] = ctype
		}

		if clength, ok := headers["CONTENT_LENGTH"]; ok {
			httpheader["Content-Length"] = clength
		}
	}

	//read the cookies
	cookies := readCookies(httpheader)

	req := Request{
		Method:     method,
		RawURL:     rawurl,
		URL:        url_,
		Proto:      proto,
		Host:       host,
		UserAgent:  useragent,
		Body:       body,
		Headers:    httpheader,
		RemoteAddr: remoteAddr,
		RemotePort: remotePort,
		Cookie:     cookies,
	}

	return &req
}

func parseForm(m map[string][]string, query string) (err error) {
	for _, kv := range strings.Split(query, "&") {
		kvPair := strings.SplitN(kv, "=", 2)

		var key, value string
		var e error
		key, e = url.QueryUnescape(kvPair[0])
		if e == nil && len(kvPair) > 1 {
			value, e = url.QueryUnescape(kvPair[1])
		}
		if e != nil {
			err = e
		}

		vec, ok := m[key]
		if !ok {
			vec = []string{}
		}
		m[key] = append(vec, value)
	}

	return
}

// ParseForm parses the request body as a form for POST requests, or the raw query for GET requests.
// It is idempotent.
func (r *Request) parseParams() (err error) {
	if r.Params != nil {
		return
	}
	r.FullParams = make(map[string][]string)
	queryParams := r.URL.RawQuery
	var bodyParams string
	switch r.Method {
	case "POST":
		if r.Body == nil {
			return errors.New("missing form body")
		}

		ct := r.Headers.Get("Content-Type")
		switch strings.SplitN(ct, ";", 2)[0] {
		case "text/plain", "application/x-www-form-urlencoded", "":
			var b []byte
			if b, err = ioutil.ReadAll(r.Body); err != nil {
				return err
			}
			bodyParams = string(b)
		case "application/json":
			//if we get JSON, do the best we can to convert it to a map[string]string
			//we make the body available as r.ParamData
			var b []byte
			if b, err = ioutil.ReadAll(r.Body); err != nil {
				return err
			}
			r.ParamData = b
			r.Params = map[string]string{}
			json.Unmarshal(b, r.Params)
		case "multipart/form-data":
			_, params, _ := mime.ParseMediaType(ct)
			boundary, ok := params["boundary"]
			if !ok {
				return errors.New("Missing Boundary")
			}

			reader := multipart.NewReader(r.Body, boundary)
			r.Files = make(map[string]filedata)
			for {
				part, err := reader.NextPart()
				if part == nil && err == io.EOF {
					break
				}

				if err != nil {
					return err
				}

				//read the data
				data, _ := ioutil.ReadAll(part)
				//check for the 'filename' param
				v := part.Header.Get("Content-Disposition")
				if v == "" {
					continue
				}
				name := part.FormName()
				d, params, _ := mime.ParseMediaType(v)
				if d != "form-data" {
					continue
				}
				if params["filename"] != "" {
					r.Files[name] = filedata{params["filename"], data}
				} else {
					var params []string = r.FullParams[name]
					params = append(params, string(data))
					r.FullParams[name] = params
				}

			}
		default:
			return &badStringError{"unknown Content-Type", ct}
		}
	}

	if queryParams != "" {
		err = parseForm(r.FullParams, queryParams)
		if err != nil {
			return err
		}
	}

	if bodyParams != "" {
		err = parseForm(r.FullParams, bodyParams)
		if err != nil {
			return err
		}
	}

	r.Params = flattenParams(r.FullParams)
	return nil
}

func (r *Request) HasFile(name string) bool {
	if r.Files == nil || len(r.Files) == 0 {
		return false
	}
	_, ok := r.Files[name]
	return ok
}

func writeTo(s string, val reflect.Value) error {
	switch v := val; v.Kind() {
	// if we're writing to an interace value, just set the byte data
	// TODO: should we support writing to a pointer?
	case reflect.Interface:
		v.Set(reflect.ValueOf(s))
	case reflect.Bool:
		if strings.ToLower(s) == "false" || s == "0" {
			v.SetBool(false)
		} else {
			v.SetBool(true)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		ui, err := strconv.ParseUint(s, 0, 64)
		if err != nil {
			return err
		}
		v.SetUint(ui)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		v.SetFloat(f)

	case reflect.String:
		v.SetString(s)
	case reflect.Slice:
		typ := v.Type()
		if typ.Elem().Kind() == reflect.Uint || typ.Elem().Kind() == reflect.Uint8 || typ.Elem().Kind() == reflect.Uint16 || typ.Elem().Kind() == reflect.Uint32 || typ.Elem().Kind() == reflect.Uint64 || typ.Elem().Kind() == reflect.Uintptr {
			v.Set(reflect.ValueOf([]byte(s)))
		}
	}
	return nil
}

// matchName returns true if key should be written to a field named name.
func matchName(key, name string) bool {
	return strings.ToLower(key) == strings.ToLower(name)
}

func (r *Request) writeToContainer(val reflect.Value) error {
	switch v := val; v.Kind() {
	case reflect.Ptr:
		return r.writeToContainer(reflect.Indirect(v))
	case reflect.Interface:
		return r.writeToContainer(v.Elem())
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return errors.New("Invalid map type")
		}
		elemtype := v.Type().Elem()
		for pk, pv := range r.Params {
			mk := reflect.ValueOf(pk)
			mv := reflect.Zero(elemtype)
			writeTo(pv, mv)
			v.SetMapIndex(mk, mv)
		}
	case reflect.Struct:
		for pk, pv := range r.Params {
			//try case sensitive match
			field := v.FieldByName(pk)
			if field.IsValid() {
				writeTo(pv, field)
			}

			//try case insensitive matching
			field = v.FieldByNameFunc(func(s string) bool { return matchName(pk, s) })
			if field.IsValid() {
				writeTo(pv, field)
			}

		}
	default:
		return errors.New("Invalid container type")
	}
	return nil
}

func (r *Request) UnmarshalParams(val interface{}) error {
	if strings.HasPrefix(r.Headers.Get("Content-Type"), "application/json") {
		return json.Unmarshal(r.ParamData, val)
	} else {
		err := r.writeToContainer(reflect.ValueOf(val))
		if err != nil {
			return err
		}
	}

	return nil
}
