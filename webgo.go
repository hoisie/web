package main

import (
	"bytes";
	"io/ioutil";
	"os";
	"path";
)

func printHelp()	{ println("Commands: create, serve") }

func exists(path string) bool {
	_, err := os.Lstat(path);
  return err == nil;
}

func createProject(name string) {
	cwd := os.Getenv("PWD");
	projectDir := path.Join(cwd, name);

	if exists(projectDir) {
		println("Project directory already exists");
    os.Exit(0);
	}

	println("Creating directory ", projectDir);
	if err := os.Mkdir(projectDir, 0744); err != nil {
		println(err.String());
    os.Exit(0);
	}

	var buffer bytes.Buffer;
	buffer.WriteString(tmpl);
  
  filename := path.Join(projectDir, name+".go");
  println("Creating template ", filename);  
	if err := ioutil.WriteFile(filename, buffer.Bytes(), 0644); err != nil {
		println(err.String());
		os.Exit(0);
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
		createProject(os.Args[2])

	case "serve":
		println("serving!")

	case "help":
		printHelp()

	default:
		printHelp()
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
