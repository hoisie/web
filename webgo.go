package main

import (
  "bytes";
  "io/ioutil";
  "os";
  "path";
)

func printHelp() {
  println("Commands: create, serve");
}

func exists(path string) bool {
  _,err := os.Lstat(path);
  
  if err == nil {
    return true;
  }
  
  return false;
}

func createProject(name string) {
  cwd := os.Getenv("PWD");
  projectDir := path.Join(cwd, name);
  
  if exists(projectDir) {
    panicln("Project directory already exists");
  }
  
  println("Creating directory ", projectDir);
  err := os.Mkdir(projectDir, 0744);
  
  if err != nil {
    panicln("Failed");
  }
  
  
  filename := path.Join(projectDir, name+".go");
  println("Creating template ", filename);
  var buffer bytes.Buffer;
  buffer.WriteString(tmpl);
  err = ioutil.WriteFile(filename, buffer.Bytes(), 0644);
  
  if err != nil {
    println(err.(*os.PathError).Error.(os.Errno));
    panicln("Failed!", err.String());
  }
}

func main() {
  if len(os.Args) <= 1 {
    printHelp();
    os.Exit(0);
  }
  
  command := os.Args[1];
  
  switch command {
    case "create": 
      createProject(os.Args[2]);
    
    case "serve": println("serving!");
    
    default : printHelp();
  }
}

var tmpl = `package main

import (
  "web";
)

var urls = map[string] interface {} {
  "/(.*)" : hello,
}

func hello (val string) string {
 return "hello "+val;
}

func main() {
  web.Run(urls, "0.0.0.0:9999");
}
`
