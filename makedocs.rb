#!/usr/bin/env ruby

head = <<-HEAD
<!DOCTYPE html>
<head>
  <link rel="stylesheet" type="text/css" href="stylesheets/styles.css">
  <meta charset='utf-8'> 
</head>

<!--  Style overrrides -->
<style>
  dl dd {
    font-style: normal;
  }
  #api {
    padding-left: 20px;
  }
</style>

<body>
<div id="api">
HEAD

tail = <<-TAIL
</div>
</body>
</html>
TAIL

f = File.open("api.html", "w")
f.write(head)
f.write(%x(godoc -html github.com/hoisie/web))
f.write(tail)
f.close