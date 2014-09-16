// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"net"
	"net/http/fcgi"
	"strings"
)

func (s *Server) listenAndServeFcgi(addr string) error {
	var l net.Listener
	var err error

	//if the path begins with a "/", assume it's a unix address
	if strings.HasPrefix(addr, "/") {
		l, err = net.Listen("unix", addr)
	} else {
		l, err = net.Listen("tcp", addr)
	}

	//save the listener so it can be closed
	s.l = l

	if err != nil {
		s.Logger.Println("FCGI listen error", err.Error())
		return err
	}
	return fcgi.Serve(l, s)
}
