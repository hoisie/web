#!/usr/bin/env ruby

header = <<-HEAD
---
layout: default
title: API docs
---

<div id="api">
HEAD

footer = <<-FOOT
</div>
FOOT

puts "Updating github.com/hoisie/web"
`go get -u github.com/hoisie/web`

f = File.open("api.html", "w")
f.write(header)
f.write(%x(godoc -html github.com/hoisie/web))
f.write(footer)
f.close
