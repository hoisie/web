package web

import (
	"bytes"
	"code.google.com/p/go.net/websocket"
	"crypto/tls"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ServerConfig is configuration for server objects.
type ServerConfig struct {
	StaticDir    string
	Addr         string
	Port         int
	CookieSecret string
	RecoverPanic bool
	Profiler     bool
}

// Server represents a web.go server.
type Server struct {
	Config    *ServerConfig
	routes    []route
	muxrouter *mux.Router
	Logger    *log.Logger
	Env       map[string]interface{}
	//save the listener so it can be closed
	l net.Listener
}

func NewServer() *Server {
	return &Server{
		Config:    Config,
		muxrouter: mux.NewRouter(),
		Logger:    log.New(os.Stdout, "", log.Ldate|log.Ltime),
		Env:       map[string]interface{}{},
	}
}

func (s *Server) initServer() {
	if s.Config == nil {
		s.Config = &ServerConfig{}
	}

	if s.Logger == nil {
		s.Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	}
}

func (s *Server) Router() *mux.Router {
	if s.muxrouter == nil {
		s.muxrouter = mux.NewRouter()
	}
	return s.muxrouter
}

type route struct {
	r           string
	cr          *regexp.Regexp
	method      string
	handler     reflect.Value
	httpHandler http.Handler
}

func (s *Server) addRoute(r string, method string, handler interface{}) {
	if s.muxrouter == nil {
		s.muxrouter = mux.NewRouter()
	}
	switch f := handler.(type) {
	case func(http.ResponseWriter, *http.Request):
		s.muxrouter.NewRoute().Path(r).HandlerFunc(f).Methods(method)
	default:
		log.Println("ERROR: ", reflect.TypeOf(handler))
		panic("incorrect handler signature: ")
	}
}

/*
func webio_handler(s *Server, f1 func(*Context, string)) func(http.ResponseWriter, *http.Request) {
	f2 := func(w http.ResponseWriter, r *http.Request) {
		s.ServeHTTP(w, r)
	}
	return f2
}
*/

// ServeHTTP is the interface method for Go's http server package
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.muxrouter.ServeHTTP(w, r)
}

// Get adds a handler for the 'GET' http method for server s.
func (s *Server) Get(route string, handler interface{}) {
	s.addRoute(route, "GET", handler)
}

// Post adds a handler for the 'POST' http method for server s.
func (s *Server) Post(route string, handler interface{}) {
	s.addRoute(route, "POST", handler)
}

// Put adds a handler for the 'PUT' http method for server s.
func (s *Server) Put(route string, handler interface{}) {
	s.addRoute(route, "PUT", handler)
}

// Delete adds a handler for the 'DELETE' http method for server s.
func (s *Server) Delete(route string, handler interface{}) {
	s.addRoute(route, "DELETE", handler)
}

// Match adds a handler for an arbitrary http method for server s.
func (s *Server) Match(method string, route string, handler interface{}) {
	s.addRoute(route, method, handler)
}

//Adds a custom handler. Only for webserver mode. Will have no effect when running as FCGI or SCGI.
func (s *Server) Handler(route string, method string, httpHandler http.Handler) {
	s.addRoute(route, method, httpHandler)
}

//Adds a handler for websockets. Only for webserver mode. Will have no effect when running as FCGI or SCGI.
func (s *Server) Websocket(route string, httpHandler websocket.Handler) {
	s.addRoute(route, "GET", httpHandler)
}

// Run starts the web application and serves HTTP requests for s
func (s *Server) Run(addr string) {
	s.initServer()

	if s.muxrouter == nil {
		s.muxrouter = mux.NewRouter()
	}
	if s.Config.Profiler {
		s.muxrouter.Handle("/debug/pprof", http.HandlerFunc(pprof.Index))
		s.muxrouter.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		s.muxrouter.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		s.muxrouter.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		s.muxrouter.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		s.muxrouter.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		s.muxrouter.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		s.muxrouter.Handle("/debug/pprof/block", pprof.Handler("block"))
	}
	s.muxrouter.Handle("/", s)

	s.Logger.Printf("serving %s\n", addr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
	s.l = l
	err = http.Serve(s.l, s.muxrouter)
	s.l.Close()
}

func (s *Server) Shutdown() {
	s.l.Close()
	//TODO: Existing connections as well must be closed gracefully
}

// RunFcgi starts the web application and serves FastCGI requests for s.
func (s *Server) RunFcgi(addr string) {
	s.initServer()
	s.Logger.Printf("serving fcgi %s\n", addr)
	s.listenAndServeFcgi(addr)
}

// RunScgi starts the web application and serves SCGI requests for s.
func (s *Server) RunScgi(addr string) {
	s.initServer()
	s.Logger.Printf("serving scgi %s\n", addr)
	s.listenAndServeScgi(addr)
}

// RunTLS starts the web application and serves HTTPS requests for s.
func (s *Server) RunTLS(addr string, config *tls.Config) error {
	s.initServer()
	mux := http.NewServeMux()
	mux.Handle("/", s)
	
	s.Logger.Printf("serving %s\n", addr)
	
	l, err := tls.Listen("tcp", addr, config)
	if err != nil {
		log.Fatal("Listen:", err)
		return err
	}

	s.l = l
	return http.Serve(s.l, mux)
}

// Close stops server s.
func (s *Server) Close() {
	if s.l != nil {
		s.l.Close()
	}
}

// safelyCall invokes `function` in recover block
func (s *Server) safelyCall(function reflect.Value, args []reflect.Value) (resp []reflect.Value, e interface{}) {
	defer func() {
		if err := recover(); err != nil {
			if !s.Config.RecoverPanic {
				// go back to panic
				panic(err)
			} else {
				e = err
				resp = nil
				s.Logger.Println("Handler crashed with error", err)
				for i := 1; ; i += 1 {
					_, file, line, ok := runtime.Caller(i)
					if !ok {
						break
					}
					s.Logger.Println(file, line)
				}
			}
		}
	}()
	return function.Call(args), nil
}

// tryServingFile attempts to serve a static file, and returns
// whether or not the operation is successful.
// It checks the following directories for the file, in order:
// 1) Config.StaticDir
// 2) The 'static' directory in the parent directory of the executable.
// 3) The 'static' directory in the current working directory
func (s *Server) tryServingFile(name string, req *http.Request, w http.ResponseWriter) bool {
	//try to serve a static file
	if s.Config.StaticDir != "" {
		staticFile := path.Join(s.Config.StaticDir, name)
		if fileExists(staticFile) {
			http.ServeFile(w, req, staticFile)
			return true
		}
	} else {
		for _, staticDir := range defaultStaticDirs {
			staticFile := path.Join(staticDir, name)
			if fileExists(staticFile) {
				http.ServeFile(w, req, staticFile)
				return true
			}
		}
	}
	return false
}

func (s *Server) logRequest(r *http.Request, sTime time.Time) {
	//log the request
	var logEntry bytes.Buffer
	req := r
	requestPath := req.URL.Path

	duration := time.Now().Sub(sTime)
	var client string

	// We suppose RemoteAddr is of the form Ip:Port as specified in the Request
	// documentation at http://golang.org/pkg/net/http/#Request
	pos := strings.LastIndex(req.RemoteAddr, ":")
	if pos > 0 {
		client = req.RemoteAddr[0:pos]
	} else {
		client = req.RemoteAddr
	}

	fmt.Fprintf(&logEntry, "%s - %s %s - %v", client, req.Method, requestPath, duration)

	/*
	if len(ctx.Params) > 0 {
		fmt.Fprintf(&logEntry, " - Params: %v\n", ctx.Params)
	}
	*/

	s.Logger.Print(logEntry.String())
}

// SetLogger sets the logger for server s
func (s *Server) SetLogger(logger *log.Logger) {
	s.Logger = logger
}
