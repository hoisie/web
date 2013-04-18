package web

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
)

func (s *Server) createServeMux(addr string) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/", s)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Listen:", err)
	}
	s.l = l

	return mux, err
}

//Runs the web application and serves http requests
func (s *Server) Run(addr string) {
	s.initServer()

	mux := http.NewServeMux()
	mux.Handle("/", s)

	s.Logger.Printf("web.go serving %s\n", addr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
	s.l = l
	err = http.Serve(s.l, mux)
	s.l.Close()
}

//Runs the secure web application and serves https requests
func (s *Server) RunSecure(addr string, config tls.Config) error {
	s.initServer()
	mux := http.NewServeMux()
	mux.Handle("/", s)

	l, err := tls.Listen("tcp4", addr, &config)
	if err != nil {
		return err
	}

	s.l = l
	return http.Serve(s.l, mux)
}
func (s *Server) RunTLS(addr string, cert string, key string) {
	s.initServer()

	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/", s)

	s.Logger.Printf("web.go serving %s\n", addr)
	/*
	   l, err := net.Listen("tcp", addr)
	   if err != nil {
	       log.Fatal("ListenAndServe:", err)
	   }
	   s.l = l
	   err = http.Serve(s.l, mux)
	   s.l.Close()
	*/
	err := http.ListenAndServeTLS(addr, cert, key, mux)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}

}

//Runs the web application and serves http requests
func Run(addr string) {
	mainServer.Run(addr)
}

//Runs the secure web application and serves https requests
func RunSecure(addr string, config tls.Config) {
	mainServer.RunSecure(addr, config)
}

func (s *Server) runTLS(addr, certFile, keyFile string) {
	s.initServer()

	mux, err := s.createServeMux(addr)
	s.Logger.Printf("web.go serving with TLS %s\n", addr)

	srv := &http.Server{Handler: mux}

	config := &tls.Config{}
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)

	if err != nil {
		log.Fatal("TLS error:", err)
	}

	tlsListener := tls.NewListener(s.l, config)
	err = srv.Serve(tlsListener)
	s.l.Close()
}

func RunTLS(addr, certFile, keyFile string) {
	mainServer.runTLS(addr, certFile, keyFile)
}
