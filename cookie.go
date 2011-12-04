// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
    "bytes"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "sort"
    "strings"
    "time"
)

func sanitizeName(n string) string {
    n = strings.Replace(n, "\n", "-", -1)
    n = strings.Replace(n, "\r", "-", -1)
    return n
}

func sanitizeValue(v string) string {
    v = strings.Replace(v, "\n", " ", -1)
    v = strings.Replace(v, "\r", " ", -1)
    v = strings.Replace(v, ";", " ", -1)
    return v
}

func isCookieByte(c byte) bool {
    switch true {
    case c == 0x21, 0x23 <= c && c <= 0x2b, 0x2d <= c && c <= 0x3a,
        0x3c <= c && c <= 0x5b, 0x5d <= c && c <= 0x7e:
        return true
    }
    return false
}

func isSeparator(c byte) bool {
    switch c {
    case '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}', ' ', '\t':
        return true
    }
    return false
}
func isChar(c byte) bool  { return 0 <= c && c <= 127 }
func isCtl(c byte) bool   { return (0 <= c && c <= 31) || c == 127 }
func isToken(c byte) bool { return isChar(c) && !isCtl(c) && !isSeparator(c) }

func parseCookieValue(raw string) (string, bool) {
    raw = unquoteCookieValue(raw)
    for i := 0; i < len(raw); i++ {
        if !isCookieByte(raw[i]) {
            return "", false
        }
    }
    return raw, true
}

func unquoteCookieValue(v string) string {
    if len(v) > 1 && v[0] == '"' && v[len(v)-1] == '"' {
        return v[1 : len(v)-1]
    }
    return v
}

func isCookieNameValid(raw string) bool {
    for _, c := range raw {
        if !isToken(byte(c)) {
            return false
        }
    }
    return true
}

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
        if _, offset := c.Expires.Zone(); offset > 0 {
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
func ReadCookies(h http.Header) []*http.Cookie {
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
    for _,line := range unparsedLines  {
        h.Set("Cookie", line)
    }
    return cookies
}
