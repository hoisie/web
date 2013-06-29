package web

import (
    "bytes"
    "encoding/base64"
    "errors"
    "net/http"
    "net/url"
    "os"
    "regexp"
    "strings"
    "time"
)

// internal utility methods
func webTime(t time.Time) string {
    ftime := t.Format(time.RFC1123)
    if strings.HasSuffix(ftime, "UTC") {
        ftime = ftime[0:len(ftime)-3] + "GMT"
    }
    return ftime
}

func dirExists(dir string) bool {
    d, e := os.Stat(dir)
    switch {
    case e != nil:
        return false
    case !d.IsDir():
        return false
    }

    return true
}

func fileExists(dir string) bool {
    info, err := os.Stat(dir)
    if err != nil {
        return false
    }

    return !info.IsDir()
}

// Urlencode is a helper method that converts a map into URL-encoded form data.
// It is a useful when constructing HTTP POST requests.
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

var slugRegex = regexp.MustCompile(`(?i:[^a-z0-9\-_])`)

// Slug is a helper function that returns the URL slug for string s.
// It's used to return clean, URL-friendly strings that can be
// used in routing.
func Slug(s string, sep string) string {
    if s == "" {
        return ""
    }
    slug := slugRegex.ReplaceAllString(s, sep)
    if slug == "" {
        return ""
    }
    quoted := regexp.QuoteMeta(sep)
    sepRegex := regexp.MustCompile("(" + quoted + "){2,}")
    slug = sepRegex.ReplaceAllString(slug, sep)
    sepEnds := regexp.MustCompile("^" + quoted + "|" + quoted + "$")
    slug = sepEnds.ReplaceAllString(slug, "")
    return strings.ToLower(slug)
}

// NewCookie is a helper method that returns a new http.Cookie object.
// Duration is specified in seconds. If the duration is zero, the cookie is permanent.
// This can be used in conjunction with ctx.SetCookie.
func NewCookie(name string, value string, age int64) *http.Cookie {
    var utctime time.Time
    if age == 0 {
        // 2^31 - 1 seconds (roughly 2038)
        utctime = time.Unix(2147483647, 0)
    } else {
        utctime = time.Unix(time.Now().Unix()+age, 0)
    }
    return &http.Cookie{Name: name, Value: value, Expires: utctime}
}

// GetBasicAuth is a helper method of *Context that returns the decoded
// user and password from the *Context's authorization header
func (ctx *Context) GetBasicAuth() (string, string, error) {
    authHeader := ctx.Request.Header["Authorization"][0]
    authString := strings.Split(string(authHeader), " ")
    if authString[0] != "Basic" {
        return "", "", errors.New("Not Basic Authentication")
    }
    decodedAuth, err := base64.StdEncoding.DecodeString(authString[1])
    if err != nil {
        return "", "", err
    }
    authSlice := strings.Split(string(decodedAuth), ":")
    if len(authSlice) != 2 {
        return "", "", errors.New("Error delimiting authString into username/password. Malformed input: " + authString[1])
    }
    return authSlice[0], authSlice[1], nil
}
