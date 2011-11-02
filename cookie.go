// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
    "bytes"
    "fmt"
    "http"
    "io"
    "sort"
    "strings"
    "time"
    "url"
)

// writeSetCookies writes the wire representation of the set-cookies
// to w. Each cookie is written on a separate "Set-Cookie: " line.
// This choice is made because HTTP parsers tend to have a limit on
// line-length, so it seems safer to place cookies on separate lines.
func writeSetCookies(w io.Writer, kk []*http.Cookie) error {
    if kk == nil {
        return nil
    }
    lines := make([]string, 0, len(kk))
    var b bytes.Buffer
    for _, c := range kk {
        b.Reset()
        // TODO(petar): c.Value (below) should be unquoted if it is recognized as quoted
        fmt.Fprintf(&b, "%s=%s", http.CanonicalHeaderKey(c.Name), c.Value)
        if len(c.Path) > 0 {
            fmt.Fprintf(&b, "; Path=%s", url.QueryEscape(c.Path))
        }
        if len(c.Domain) > 0 {
            fmt.Fprintf(&b, "; Domain=%s", url.QueryEscape(c.Domain))
        }
        if len(c.Expires.Zone) > 0 {
            fmt.Fprintf(&b, "; Expires=%s", c.Expires.Format(time.RFC1123))
        }
        if c.MaxAge >= 0 {
            fmt.Fprintf(&b, "; Max-Age=%d", c.MaxAge)
        }
        if c.HttpOnly {
            fmt.Fprintf(&b, "; HttpOnly")
        }
        if c.Secure {
            fmt.Fprintf(&b, "; Secure")
        }
        lines = append(lines, "Set-Cookie: "+b.String()+"\r\n")
    }
    sort.Strings(lines)
    for _, l := range lines {
        if _, err := io.WriteString(w, l); err != nil {
            return err
        }
    }
    return nil
}

// writeCookies writes the wire representation of the cookies
// to w. Each cookie is written on a separate "Cookie: " line.
// This choice is made because HTTP parsers tend to have a limit on
// line-length, so it seems safer to place cookies on separate lines.
func writeCookies(w io.Writer, kk []*http.Cookie) error {
    lines := make([]string, 0, len(kk))
    var b bytes.Buffer
    for _, c := range kk {
        b.Reset()
        n := c.Name
        // TODO(petar): c.Value (below) should be unquoted if it is recognized as quoted
        fmt.Fprintf(&b, "%s=%s", http.CanonicalHeaderKey(n), c.Value)
        if len(c.Path) > 0 {
            fmt.Fprintf(&b, "; $Path=%s", url.QueryEscape(c.Path))
        }
        if len(c.Domain) > 0 {
            fmt.Fprintf(&b, "; $Domain=%s", url.QueryEscape(c.Domain))
        }
        if c.HttpOnly {
            fmt.Fprintf(&b, "; $HttpOnly")
        }
        lines = append(lines, "Cookie: "+b.String()+"\r\n")
    }
    sort.Strings(lines)
    for _, l := range lines {
        if _, err := io.WriteString(w, l); err != nil {
            return err
        }
    }
    return nil
}

// readCookies parses all "Cookie" values from
// the header h, removes the successfully parsed values from the 
// "Cookie" key in h and returns the parsed Cookies.
func readCookies(h http.Header) []*http.Cookie {
    cookies := []*http.Cookie{}
    lines, ok := h["Cookie"]
    if !ok {
        return cookies
    }
    unparsedLines := []string{}
    for _, line := range lines {
        parts := strings.Split(strings.TrimSpace(line), ";")
        if len(parts) == 1 && parts[0] == "" {
            continue
        }
        // Per-line attributes
        var lineCookies = make(map[string]string)
        var path string
        var domain string
        var httponly bool
        for i := 0; i < len(parts); i++ {
            parts[i] = strings.TrimSpace(parts[i])
            if len(parts[i]) == 0 {
                continue
            }
            attr, val := parts[i], ""
            var err error
            if j := strings.Index(attr, "="); j >= 0 {
                attr, val = attr[:j], attr[j+1:]
                val, err = url.QueryUnescape(val)
                if err != nil {
                    continue
                }
            }
            switch strings.ToLower(attr) {
            case "$httponly":
                httponly = true
            case "$domain":
                domain = val
                // TODO: Add domain parsing
            case "$path":
                path = val
                // TODO: Add path parsing
            default:
                lineCookies[attr] = val
            }
        }
        if len(lineCookies) == 0 {
            unparsedLines = append(unparsedLines, line)
        }
        for n, v := range lineCookies {
            cookies = append(cookies, &http.Cookie{
                Name:     n,
                Value:    v,
                Path:     path,
                Domain:   domain,
                HttpOnly: httponly,
                MaxAge:   -1,
                Raw:      line,
            })
        }
    }
    if len(unparsedLines) > 0 {
        h["Cookie"] = unparsedLines
    } else {
        delete(h, "Cookie")
    }
    return cookies
}
