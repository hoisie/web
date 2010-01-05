package web

import (
    "io"
    "os"
    "path"
    "utf8"
)

var contentByExt = map[string]string{
    ".css": "text/css",
    ".gif": "image/gif",
    ".html": "text/html; charset=utf-8",
    ".htm": "text/html; charset=utf-8",
    ".jpg": "image/jpeg",
    ".js": "application/x-javascript",
    ".pdf": "application/pdf",
    ".png": "image/png",
}

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

func serveFile(ctx *Context, name string) {
    f, err := os.Open(name, os.O_RDONLY, 0)

    if err != nil {
        ctx.Abort(404, "Invalid file")
        return
    }

    defer f.Close()
    ext := path.Ext(name)

    if ctype, ok := contentByExt[ext]; ok {
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
        ctx.Write(b)
    }
    io.Copy(ctx, f)
}
