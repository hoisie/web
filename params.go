// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"strconv"
)

type Params map[string]string

// Get a parameter. Panics if not found. Panic object is a WebError with status
// 400.
func (p Params) GetString(key string) string {
	val, ok := p[key]
	if !ok {
		panic(WebError{400, "Required parameter " + key + " missing"})
	}
	return val
}

// Get a parameter as an integer value. Panics if not found or not a legal
// integer. Panic object is a WebError with status 400.
func (p Params) GetInt(key string) int {
	i, err := strconv.Atoi(p.GetString(key))
	if err != nil {
		panic(WebError{400, "Illegal integer parameter " + key})
	}
	return i
}
