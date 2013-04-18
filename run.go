package web

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/pprof"
)

// Create a http.Handler from this server that also provides some debug
// resources
func (s *Server) createDebugHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/", s)
	return mux
}

// Runs the web application and serves http requests
func (s *Server) Run(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		s.Logger.Print("run web.go: ", err)
		return err
	}
	defer l.Close()
	s.Logger.Printf("web.go serving %s\n", addr)
	s.l = l
	return http.Serve(l, s)
}

//Runs the secure web application and serves https requests
func (s *Server) RunSecure(addr string, config tls.Config) error {
	l, err := tls.Listen("tcp4", addr, &config)
	if err != nil {
		s.Logger.Print("run web.go: ", err)
		return err
	}
	defer l.Close()
	s.l = l
	return http.Serve(l, s)
}

func (s *Server) RunTLS(addr string, cert string, key string) error {
	debugh := s.createDebugHandler()
	s.Logger.Print("web.go serving ", addr)
	err := http.ListenAndServeTLS(addr, cert, key, debugh)
	if err != nil {
		s.Logger.Print("ListenAndServe: ", err)
		return err
	}
	return nil
}

func (s *Server) runTLS(addr, certFile, keyFile string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		s.Logger.Print("start web.go: ", err)
		return err
	}
	defer l.Close()
	s.l = l
	debugh := s.createDebugHandler()
	s.Logger.Print("web.go serving with TLS ", addr)

	srv := &http.Server{Handler: debugh}

	config := &tls.Config{}
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)

	if err != nil {
		s.Logger.Print("TLS error: ", err)
		return err
	}

	tlsListener := tls.NewListener(s.l, config)
	return srv.Serve(tlsListener)
}

//Runs the web application and serves http requests
func Run(addr string) {
	mainServer.Run(addr)
}

//Runs the secure web application and serves https requests
func RunSecure(addr string, config tls.Config) {
	mainServer.RunSecure(addr, config)
}

func RunTLS(addr, certFile, keyFile string) {
	mainServer.runTLS(addr, certFile, keyFile)
}
