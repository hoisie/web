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
`go get -u -a github.com/hoisie/web`
if $?.exitstatus != 0
  puts "`go get -u -a github.com/hoisie/web` failed"
end

go_path = %x(go env GOPATH).to_s.strip

doc_html = %x(godoc -html -timestamps=false github.com/hoisie/web)

# get the short-hash of the current branch (for Github source links)
webgo_root = "#{go_path}/src/github.com/hoisie/web"
git_head = %x(cd #{webgo_root} && git log --pretty=format:'%h' -n 1 master).to_s.strip
puts "Found web.go revision #{git_head} in #{webgo_root}"

# replace local source links with Github links
doc_html.gsub!(%r(/target/(.*)\?s=(\d+):(\d+)#L\d+)) do |match|
  start_line = File.read("#{webgo_root}/#{$1}")[0..$2.to_i].lines.count
  end_line = File.read("#{webgo_root}/#{$1}")[0..$3.to_i].lines.count
  "https://github.com/hoisie/web/blob/#{git_head}/#{$1}#L#{start_line}-#{end_line}"
end

puts "Writing to api.html"
f = File.open("api.html", "w")
f.write(header)
f.write(doc_html)
f.write(footer)
f.close
