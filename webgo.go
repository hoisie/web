package main

import (
	"bytes";
	"fmt";
	"io/ioutil";
	"os";
	"path";
	"strings";
)

func clean(line string) string {
	var i int;
	for i = 0; i < len(line); i++ {
		if line[i] == ';' {
			break
		}
	}
	line = line[0:i];

	return strings.TrimSpace(line);
}

type LineError string

func (err LineError) String() string	{ return fmt.Sprintf("Error parsing line %q", err) }

func parseIni(contents string) (map[string]map[string]string, os.Error) {
	lines := strings.Split(contents, "\n", 0);

	parsed := make(map[string]map[string]string);
	var cur *map[string]string;

	for _, line := range (lines) {
		cleaned := clean(line);
		if cleaned == "" {
			continue
		} else if cleaned[0] == '[' && cleaned[len(cleaned)-1] == ']' {
			name := cleaned[1 : len(cleaned)-1];
			ns := make(map[string]string);
			parsed[name] = ns;
			cur = &ns;
		} else if strings.Index(cleaned, "=") != -1 {
			a := strings.Split(line, "=", 0);
			key, value := strings.TrimSpace(a[0]), strings.TrimSpace(a[1]);
			(*cur)[key] = value;
		} else {
			return nil, LineError(line)
		}
	}

	return parsed, nil;
}

func ParseString(contents string) (map[string]map[string]string, os.Error) {
	return parseIni(contents)
}

func ParseFile(filename string) (map[string]map[string]string, os.Error) {
	contents, err := ioutil.ReadFile(filename);
	if err != nil {
		return nil, err
	}

	return parseIni(string(contents));
}


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
