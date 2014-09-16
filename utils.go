// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"net/url"
)

// URL encode a key/value map to a query string
func Urlencode(data map[string]string) string {
	var buf bytes.Buffer
	for k, v := range data {
		buf.WriteString(url.QueryEscape(k))
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(v))
		buf.WriteByte('&')
	}
	s := buf.String()
	return s[0 : len(s)-1]
}
