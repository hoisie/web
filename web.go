// Package web is a lightweight web framework for Go. It's ideal for
// writing simple, performant backend web services.
package web

import (
    "bytes"
    "code.google.com/p/go.net/websocket"
    "crypto/hmac"
    "crypto/sha1"
    "crypto/tls"
    "encoding/base64"
    "fmt"
    "io/ioutil"
    "log"
    "mime"
    "net/http"
    "os"
    "path"
    "strconv"
    "strings"
    "time"
)

// A Context object is created for every incoming HTTP request, and is
// passed to handlers as an optional first argument. It provides information
// about the request, including the http.Request object, the GET and POST params,
// and acts as a Writer for the response.
/*
type Context struct {
    Request *http.Request
    Params  map[string]string
    Server  *Server
    http.ResponseWriter
}
*/

// WriteString writes string data into the response object.
func WriteString(w http.ResponseWriter, content string) {
	w.Write([]byte(content))
}

// Abort is a helper method that sends an HTTP header and an optional
// body. It is useful for returning 4xx or 5xx errors.
// Once it has been called, any return value from the handler will
// not be written to the response.
func Abort(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	w.Write([]byte(body))
}

// Redirect is a helper method for 3xx redirects.
func Redirect(w http.ResponseWriter, status int, url_ string) {
	w.Header().Set("Location", url_)
	w.WriteHeader(status)
	w.Write([]byte("Redirecting to: " + url_))
}

// Notmodified writes a 304 HTTP response
func NotModified(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotModified)
}

// NotFound writes a 404 HTTP response
func NotFound(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(message))
}

//Unauthorized writes a 401 HTTP response
func Unauthorized(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
}

//Forbidden writes a 403 HTTP response
func Forbidden(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
}

// ContentType sets the Content-Type header for an HTTP response.
// For example, ContentType(w, "json") sets the content-type to "application/json"
// If the supplied value contains a slash (/) it is set as the Content-Type
// verbatim. The return value is the content type as it was
// set, or an empty string if none was found.
func ContentType(w http.ResponseWriter, val string) string {
    var ctype string
    if strings.ContainsRune(val, '/') {
        ctype = val
    } else {
        if !strings.HasPrefix(val, ".") {
            val = "." + val
        }
        ctype = mime.TypeByExtension(val)
    }
    if ctype != "" {
		w.Header().Set("Content-Type", ctype)
    }
    return ctype
}

// SetHeader sets a response header. If `unique` is true, the current value
// of that header will be overwritten . If false, it will be appended.
func SetHeader(w http.ResponseWriter, hdr string, val string, unique bool) {
    if unique {
		w.Header().Set(hdr, val)
    } else {
		w.Header().Add(hdr, val)
    }
}

// SetCookie adds a cookie header to the response.
func SetCookie(w http.ResponseWriter, cookie *http.Cookie) {
	SetHeader(w, "Set-Cookie", cookie.String(), false)
}

func getCookieSig(key string, val []byte, timestamp string) string {
    hm := hmac.New(sha1.New, []byte(key))

    hm.Write(val)
    hm.Write([]byte(timestamp))

    hex := fmt.Sprintf("%02x", hm.Sum(nil))
    return hex
}

func SetSecureCookie(w http.ResponseWriter, s *Server, name string, val string, age int64) {
    //base64 encode the val
	if len(s.Config.CookieSecret) == 0 {
		s.Logger.Println("Secret Key for secure cookies has not been set. Please assign a cookie secret to web.Config.CookieSecret.")
        return
    }
    var buf bytes.Buffer
    encoder := base64.NewEncoder(base64.StdEncoding, &buf)
    encoder.Write([]byte(val))
    encoder.Close()
    vs := buf.String()
    vb := buf.Bytes()
    timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sig := getCookieSig(s.Config.CookieSecret, vb, timestamp)
    cookie := strings.Join([]string{vs, timestamp, sig}, "|")
	SetCookie(w, NewCookie(name, cookie, age))
}

func GetSecureCookie(r *http.Request, s *Server, name string) (string, bool) {
	for _, cookie := range r.Cookies() {
        if cookie.Name != name {
            continue
        }

        parts := strings.SplitN(cookie.Value, "|", 3)

        val := parts[0]
        timestamp := parts[1]
        sig := parts[2]

		if getCookieSig(s.Config.CookieSecret, []byte(val), timestamp) != sig {
            return "", false
        }

        ts, _ := strconv.ParseInt(timestamp, 0, 64)

        if time.Now().Unix()-31*86400 > ts {
            return "", false
        }

        buf := bytes.NewBufferString(val)
        encoder := base64.NewDecoder(base64.StdEncoding, buf)

        res, _ := ioutil.ReadAll(encoder)
        return string(res), true
    }
    return "", false
}

var defaultStaticDirs []string

func init() {
    //find the location of the exe file
    wd, _ := os.Getwd()
    arg0 := path.Clean(os.Args[0])
    var exeFile string
    if strings.HasPrefix(arg0, "/") {
        exeFile = arg0
    } else {
        //TODO for robustness, search each directory in $PATH
        exeFile = path.Join(wd, arg0)
    }
    parent, _ := path.Split(exeFile)
    defaultStaticDirs = append(defaultStaticDirs, path.Join(parent, "static"))
    defaultStaticDirs = append(defaultStaticDirs, path.Join(wd, "static"))
    return
}

// Run starts the web application and serves HTTP requests for the main server.
func Run(addr string) {
    mainServer.Run(addr)
}

// RunTLS starts the web application and serves HTTPS requests for the main server.
func RunTLS(addr string, config *tls.Config) {
    mainServer.RunTLS(addr, config)
}

// RunScgi starts the web application and serves SCGI requests for the main server.
func RunScgi(addr string) {
    mainServer.RunScgi(addr)
}

// RunFcgi starts the web application and serves FastCGI requests for the main server.
func RunFcgi(addr string) {
    mainServer.RunFcgi(addr)
}

// Close stops the main server.
func Close() {
    mainServer.Close()
}

// Get adds a handler for the 'GET' http method in the main server.
func Get(route string, handler interface{}) {
    mainServer.Get(route, handler)
}

// Post adds a handler for the 'POST' http method in the main server.
func Post(route string, handler interface{}) {
    mainServer.addRoute(route, "POST", handler)
}

// Put adds a handler for the 'PUT' http method in the main server.
func Put(route string, handler interface{}) {
    mainServer.addRoute(route, "PUT", handler)
}

// Delete adds a handler for the 'DELETE' http method in the main server.
func Delete(route string, handler interface{}) {
    mainServer.addRoute(route, "DELETE", handler)
}

// Match adds a handler for an arbitrary http method in the main server.
func Match(method string, route string, handler interface{}) {
    mainServer.addRoute(route, method, handler)
}

//Adds a custom handler. Only for webserver mode. Will have no effect when running as FCGI or SCGI.
func Handler(route string, method string, httpHandler http.Handler) {
    mainServer.Handler(route, method, httpHandler)
}

//Adds a handler for websockets. Only for webserver mode. Will have no effect when running as FCGI or SCGI.
func Websocket(route string, httpHandler websocket.Handler) {
    mainServer.Websocket(route, httpHandler)
}

// SetLogger sets the logger for the main server.
func SetLogger(logger *log.Logger) {
    mainServer.Logger = logger
}

// Config is the configuration of the main server.
var Config = &ServerConfig{
    RecoverPanic: true,
}

var mainServer = NewServer()
