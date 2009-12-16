package main

import (
    "bytes";
    "fmt";
    "ini";
    "io";
    "io/ioutil";
    "os";
    "path";
    "template";
)

func writeTemplate(tmplString string, data interface{}, filename string) os.Error {
    var err os.Error;
    tmpl := template.New(nil);
    tmpl.SetDelims("{{", "}}");

    if err = tmpl.Parse(tmplString); err != nil {
        return err
    }

    var buf bytes.Buffer;

    tmpl.Execute(data, &buf);

    if err := ioutil.WriteFile(filename, buf.Bytes(), 0644); err != nil {
        return err
    }

    return nil;
}

func printHelp() { println("Commands: create, serve") }

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

    appfile := path.Join(projectDir, name+".go");
    println("Creating application file", appfile);
    writeTemplate(apptmpl, map[string]string{"app": name}, appfile);

    inifile := path.Join(projectDir, "default.ini");
    println("Creating config file", inifile);
    writeTemplate(initmpl, map[string]string{"app": name}, inifile);

}

func getOutput(command string, args []string) (string, os.Error) {
    r, w, err := os.Pipe();
    if err != nil {
        return "", err
    }
    args2 := make([]string, len(args)+1);
    args2[0] = command;
    copy(args2[1:], args);
    pid, err := os.ForkExec(command, args2, os.Environ(), "", []*os.File{nil, w, w});

    if err != nil {
        return "", err
    }

    w.Close();

    var b bytes.Buffer;
    io.Copy(&b, r);
    output := b.String();
    os.Wait(pid, 0);

    return output, nil;
}

func serveProject(inifile string) {
    cwd := os.Getenv("PWD");
    inifile = path.Join(cwd, inifile);
    datadir := path.Join(cwd, "data/");

    if !exists(datadir) {
        if err := os.Mkdir(datadir, 0744); err != nil {
            println(err.String());
            os.Exit(0);
        }
    }

    config, err := ini.ParseFile(inifile);

    if err != nil {
        println("Error parsing config", err.String())
    }

    app := config["main"]["application"];

    println("Serving application", app);

    address := fmt.Sprintf("%s:%s", config["main"]["bind_address"], config["main"]["port"]);
    gobin := os.Getenv("GOBIN");

    compiler := path.Join(gobin, "8g");
    println("compiler", compiler);

    if err != nil {
        println("Error parsing config", err.String())
    } else {
        println(address)
    }

    appSrc := path.Join(cwd, app+".go");
    appObj := path.Join(datadir, app+".8");

    compoutput, err := getOutput(compiler, []string{"-o", appObj, appSrc});

    if err == nil {
        println("Compiling", compoutput)
    }

    //8g -o data/hello.8 hello.go
    //8g -I data runner.go
    //8l -o data/hello data/hello.8 data/runner.8

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
        serveProject(os.Args[2])

    case "help":
        printHelp()

    default:
        printHelp()
    }
}

var apptmpl = `package {{app}}

import (
  //"web";
)

var Routes = map[string] interface {} {
  "/(.*)" : hello,
}

func hello (val string) string {
 return "hello "+val;
}
`

var initmpl = `[main]
application = {{app}}
bind_address = 0.0.0.0
port = 9999
`
var runnertmpl = `package main

import (
        "{application}";
        "web";
)

func main() {
        web.Run(hello.Routes, {address});
}

`
