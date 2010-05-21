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
    "utf8"
)

func isText(b []byte) bool {
    for len(b) > 0 && utf8.FullRune(b) {
        rune, size := utf8.DecodeRune(b)
        if size == 1 && rune == utf8.RuneError {
            // decoding error
            return false
        }
        if 0x80 <= rune && rune <= 0x9F {
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
    hash.Write([]byte(data))
    return fmt.Sprintf("%x", hash.Sum())
}

func serveFile(ctx *Context, name string) {
    f, err := os.Open(name, os.O_RDONLY, 0)

    if err != nil {
        ctx.Abort(404, "Invalid file")
        return
    }

    defer f.Close()

    info, _ := os.Stat(name)
    //set content-length
    ctx.SetHeader("Content-Length", strconv.Itoa64(info.Size), true)

    lm := time.SecondsToLocalTime(info.Mtime_ns / 1e9)
    //set the last-modified header
    ctx.SetHeader("Last-Modified", lm.Format(time.RFC1123), true)

    //generate a simple etag with heuristic MD5(filename, size, lastmod)
    etagparts := []string{name, strconv.Itoa64(info.Size), strconv.Itoa64(info.Mtime_ns)}
    etag := fmt.Sprintf(`"%s"`, getmd5(strings.Join(etagparts, "|")))
    ctx.SetHeader("ETag", etag, true)

    ext := path.Ext(name)
    if ctype := mime.TypeByExtension(ext); ctype != "" {
        ctx.SetHeader("Content-Type", ctype, true)
    } else {
        // read first chunk to decide between utf-8 text and binary
        var buf [1024]byte
        n, _ := io.ReadFull(f, &buf)
        b := buf[0:n]
        if isText(b) {
            ctx.SetHeader("Content-Type", "text-plain; charset=utf-8", true)
        } else {
            ctx.SetHeader("Content-Type", "application/octet-stream", true) // generic binary
        }
        if ctx.Request.Method != "HEAD" {
            ctx.Write(b)
        }
    }
    if ctx.Request.Method != "HEAD" {
        io.Copy(ctx, f)
    }
}
