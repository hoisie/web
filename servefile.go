package web

import (
    "crypto/md5"
    "fmt"
    "io"
    "mime"
    "os"
    "path"
    "strconv"
    "strings"
    "time"
    "unicode/utf8"
)

func isText(b []byte) bool {
    for len(b) > 0 && utf8.FullRune(b) {
        rune, size := utf8.DecodeRune(b)
        if size == 1 && rune == utf8.RuneError {
            // decoding error
            return false
        }
        if 0x7F <= rune && rune <= 0x9F {
            return false
        }
        if rune < ' ' {
            switch rune {
            case '\n', '\r', '\t':
                // okay
            default:
                // binary garbage
                return false
            }
        }
        b = b[size:]
    }
    return true
}

func getmd5(data string) string {
    hash := md5.New()
    return fmt.Sprintf("%x", hash.Sum([]byte(data)))
}

func serveFile(ctx *Context, name string) {
    f, err := os.Open(name)

    if err != nil {
        ctx.Abort(404, "Invalid file")
        return
    }

    defer f.Close()

    info, _ := f.Stat()
    size := strconv.Itoa64(info.Size())
    mtime := strconv.Itoa64(info.ModTime().UnixNano())
    //set the last-modified header
    lm := info.ModTime().UTC()
    ctx.SetHeader("Last-Modified", webTime(&lm), true)

    //generate a simple etag with heuristic MD5(filename, size, lastmod)
    etagparts := []string{name, size, mtime}
    etag := fmt.Sprintf(`"%s"`, getmd5(strings.Join(etagparts, "|")))
    ctx.SetHeader("ETag", etag, true)

    //the first 1024 bytes of the file, used to detect content-type
    var firstChunk []byte
    ext := path.Ext(name)
    if ctype := mime.TypeByExtension(ext); ctype != "" {
        ctx.SetHeader("Content-Type", ctype, true)
    } else {
        // read first chunk to decide between utf-8 text and binary
        buf := make([]byte, 1024)
        n, _ := io.ReadFull(f, buf)
        firstChunk = buf[0:n]
        if isText(firstChunk) {
            ctx.SetHeader("Content-Type", "text/plain; charset=utf-8", true)
        } else {
            ctx.SetHeader("Content-Type", "application/octet-stream", true) // generic binary
        }
    }

    if ctx.Request.Headers.Get("If-None-Match") != "" {
        inm := ctx.Request.Headers.Get("If-None-Match")
        if inm == etag {
            ctx.NotModified()
            return
        }

    }

    if ctx.Request.Headers.Get("If-Modified-Since") != "" {
        ims := ctx.Request.Headers.Get("If-Modified-Since")
        imstime, err := time.Parse(time.RFC1123, ims)
        if err == nil && imstime.Unix() >= lm.Unix() {
            ctx.NotModified()
            return
        }
    }

    //set content-length
    ctx.SetHeader("Content-Length", size, true)
    if ctx.Request.Method != "HEAD" {
        if len(firstChunk) > 0 {
            ctx.Write(firstChunk)
        }
        io.Copy(ctx, f)
    }
}
