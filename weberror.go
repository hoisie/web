// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import ()

type WebError struct {
	Code int
	Err  string
}

func (err WebError) Error() string {
	return err.Err
}
