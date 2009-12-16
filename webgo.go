package main

import (
    "bytes"
    "fmt"
    "ini"
    "io"
    "io/ioutil"
    "os"
    "os/signal"
    "path"
    "syscall"
    "template"
)

var toolext = map[string]string{"386": "8", "amd64": "6", "arm": "5"}

func writeTemplate(tmplString string, data interface{}, filename string) os.Error {
    var err os.Error
    tmpl := template.New(nil)
    tmpl.SetDelims("{{", "}}")

    if err = tmpl.Parse(tmplString); err != nil {
        return err
    }

    var buf bytes.Buffer

    tmpl.Execute(data, &buf)

    if err := ioutil.WriteFile(filename, buf.Bytes(), 0644); err != nil {
        return err
    }

    return nil
}

func printHelp() { println("Commands: create, serve") }

func exists(path string) bool {
    _, err := os.Lstat(path)
    return err == nil
}

func create(name string) {
    cwd := os.Getenv("PWD")
    projectDir := path.Join(cwd, name)

    if exists(projectDir) {
        println("Project directory already exists")
        os.Exit(0)
    }

    println("Creating directory ", projectDir)
    if err := os.Mkdir(projectDir, 0744); err != nil {
        println(err.String())
        os.Exit(0)
    }

    appfile := path.Join(projectDir, name+".go")
    println("Creating application file", appfile)
    writeTemplate(apptmpl, map[string]string{"app": name}, appfile)

    inifile := path.Join(projectDir, "default.ini")
    println("Creating config file", inifile)
    writeTemplate(initmpl, map[string]string{"app": name}, inifile)

}

func getOutput(command string, args []string) (string, os.Error) {
    r, w, err := os.Pipe()
    if err != nil {
        return "", err
    }
    args2 := make([]string, len(args)+1)
    args2[0] = command
    copy(args2[1:], args)
    pid, err := os.ForkExec(command, args2, os.Environ(), "", []*os.File{nil, w, w})

    if err != nil {
        return "", err
    }

    w.Close()

    var b bytes.Buffer
    io.Copy(&b, r)
    output := b.String()
    os.Wait(pid, 0)

    return output, nil
}

func serve(inifile string) {
    cwd := os.Getenv("PWD")
    inifile = path.Join(cwd, inifile)
    datadir := path.Join(cwd, "data/")

    if !exists(datadir) {
        if err := os.Mkdir(datadir, 0744); err != nil {
            println(err.String())
            return
        }
    }

    config, err := ini.ParseFile(inifile)

    if err != nil {
        println("Error parsing config", err.String())
        return
    }

    app := config["main"]["application"]

    println("Serving application", app)

    address := fmt.Sprintf("%s:%s", config["main"]["bind_address"], config["main"]["port"])
    gobin := os.Getenv("GOBIN")
    goarch := os.Getenv("GOARCH")
    ext := toolext[goarch]
    compiler := path.Join(gobin,  ext+"g")
    linker := path.Join(gobin, ext+"l")

    appSrc := path.Join(cwd, app+".go")
    appObj := path.Join(datadir, app+"."+ext)

    output, err := getOutput(compiler, []string{"-o", appObj, appSrc})

    if err != nil {
        println("Error executing compiler", err.String())
        return
    }

    if output != "" {
        println("Error compiling web application")
        println(output)
        return
    }

    //generate runner.go

    runnerSrc := path.Join(datadir, "runner.go")
    runnerObj := path.Join(datadir, "runner."+ext)

    writeTemplate(runnertmpl, map[string]string{"app": app, "address": address}, runnerSrc)

    output, err = getOutput(compiler, []string{"-o", runnerObj, "-I", datadir, runnerSrc})

    if err != nil {
        println("Error Compiling", runnerSrc, err.String())
        return
    }

    if output != "" {
        println("Error compiling runner application")
        println(output)
        return
    }

    //link the web program

    obj := path.Join(cwd, app)
    output, err = getOutput(linker, []string{"-o", obj, runnerObj, appObj})

    if err != nil {
        println("Error Linking", err.String())
        return
    }

    if output != "" {
        println("Error linking")
        println(output)
        return
    }

    pid, err := os.ForkExec(obj, []string{}, os.Environ(), "", []*os.File{nil, os.Stdout, os.Stdout})

    if err == nil {
        println("Serving on address", address)
    }

    waitchan := make(chan int, 0)
    sigchan := make(chan int, 0)

    go waitProcess(waitchan, pid)

    go waitSignal(sigchan)
    select {
    case _ = <-waitchan:
        println("Server process terminated")
    case _ = <-sigchan:
        println("Received kill signal")
        syscall.Kill(pid, 9)
        os.Wait(pid, 0)
    }

}

func waitProcess(waitchan chan int, pid int) {
    os.Wait(pid, 0)
    waitchan <- 0
}

//temporary fix for being able to kill webgo process until the language is fixed
func waitSignal(sigchan chan int) {
    for true {
        sig := (<-signal.Incoming).(signal.UnixSignal)
        if sig == 2 || sig == 15 || sig == 9 {
            sigchan <- 0
            break
        }
    }
}

func clean(inifile string) {
    cwd := os.Getenv("PWD")
    inifile = path.Join(cwd, inifile)
    datadir := path.Join(cwd, "data/")

    config, err := ini.ParseFile(inifile)

    if err != nil {
        println("Error parsing config file", err.String())
        return
    }

    app := config["main"]["application"]

    if len(app) == 0 {
        println("Invalid application name")
        return
    }

    obj := path.Join(cwd, app)

    if exists(obj) {
        println("Removing", obj)
        pid, _ := os.ForkExec("/bin/rm", []string{"/bin/rm", obj}, os.Environ(), "", []*os.File{nil, os.Stdout, os.Stdout})
        os.Wait(pid, 0)
    }

    if exists(datadir) {
        println("Removing", datadir)
        pid, _ := os.ForkExec("/bin/rm", []string{"/bin/rm", "-rf", datadir}, os.Environ(), "", []*os.File{nil, os.Stdout, os.Stdout})
        os.Wait(pid, 0)
    }
}

func main() {
    if len(os.Args) <= 1 {
        printHelp()
        os.Exit(0)
    }
    inifile := "default.ini"
    command := os.Args[1]

    switch command {
    case "create":
        create(os.Args[2])

    case "serve":
        if len(os.Args) == 3 {
            inifile = os.Args[2]
        }
        serve(inifile)

    case "clean":
        if len(os.Args) == 3 {
            inifile = os.Args[2]
        }
        clean(inifile)

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
        "{{app}}";
        "web";
)

func main() {
        web.Run({{app}}.Routes, "{{address}}");
}

`
