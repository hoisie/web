// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
)

// Listen for HTTP connections
func (s *Server) Run(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()
	s.l = l
	s.Logger.Print("web.go serving ", addr)
	return http.Serve(l, s)
}

// Listen for HTTPS connections
func (s *Server) RunTLS(addr, certFile, keyFile string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()
	s.l = l
	srv := &http.Server{Handler: s}
	config := &tls.Config{}
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("opening certificate: %s", err)
	}
	tlsListener := tls.NewListener(l, config)
	s.Logger.Print("web.go serving with TLS ", addr)
	return srv.Serve(tlsListener)
}

func (s *Server) RunScgi(addr string) error {
	s.Logger.Print("web.go serving scgi ", addr)
	return s.listenAndServeScgi(addr)
}

//Runs the web application and serves fcgi requests for this Server object.
func (s *Server) RunFcgi(addr string) error {
	s.Logger.Print("web.go serving fcgi ", addr)
	return s.listenAndServeFcgi(addr)
}

//Runs the web application and serves http requests
func Run(addr string) error {
	return mainServer.Run(addr)
}

//Runs the secure web application and serves https requests
func RunTLS(addr, certFile, keyFile string) error {
	return mainServer.RunTLS(addr, certFile, keyFile)
}

//Runs the web application and serves scgi requests
func RunScgi(addr string) error {
	return mainServer.RunScgi(addr)
}

//Runs the web application by serving fastcgi requests
func RunFcgi(addr string) error {
	return mainServer.RunFcgi(addr)
}
