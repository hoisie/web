package web

import (
	"log"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
)

var (
	AppURL string
)

func launchBrowser(addr string) {
	argv := make([]string,0,4)
	AppURL = "http://" + addr + "/"
	autoclose := true

	switch runtime.GOOS {
		case "linux":
		// Linux
		//  - Check $PATH for chromium-browser
		paths := strings.Split(os.Getenv("PATH"),":",-1)
		for _,base := range paths {
			binpath := path.Join(base, "chromium-browser")
			if _,err := os.Stat(binpath); err == nil {
				argv = append(argv, binpath, "--app=" + AppURL)
			}
		}

		case "darwin":
		// Mac OS X
		//  This is not ideal... Chrome can't do application mode
		if len(argv) == 0 {
			binpath := path.Join("/Applications", "Google Chrome.app")
			if _,err := os.Stat(binpath); err == nil {
				argv = append(argv, "/usr/bin/open", AppURL)
				autoclose = false
			}
		}
	}

	if len(argv) == 0 {
		log.Fatal("Unable to find Chrome or Chromium web browser")
	}

	pid, err := os.ForkExec(argv[0], argv, os.Environ(), "", []*os.File{nil, os.Stdout, os.Stderr})
	if err != nil {
		log.Fatalf("Could not launch browser: %s\n", err)
	}
	log.Printf("Launching browser: %s (%d)\n", AppURL, pid)

	// Exit the application if we quit internally
	if autoclose {
		go func() {
			for q := false; !q; {
				select {
				//case q = <-gwa.quit:
				case sig := <-signal.Incoming:
					if usig, ok := sig.(signal.UnixSignal); ok {
						switch usig {
						// If we get ^C, we should exit
						case syscall.SIGINT:
							q = true
						// If we get ^z, we should stop
						case syscall.SIGTSTP:
							syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
						}
					}
				}
			}
			syscall.Kill(pid, syscall.SIGTERM)
			log.Printf("Killed browser (%d)\n", pid)
		}()
	} else {
		log.Printf("Press ^C to exit\n", pid)
		for q := false; !q; {
			select {
			//case q = <-gwa.quit:
			case sig := <-signal.Incoming:
				if usig, ok := sig.(signal.UnixSignal); ok {
					switch usig {
					// If we get ^C, we should exit
					case syscall.SIGINT:
						q = true
					// If we get ^z, we should stop
					case syscall.SIGTSTP:
						syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
					}
				}
			}
		}
	}

	os.Wait(pid, 0)
	log.Println("Application exited")
}

func RunApp(addr string) {
	go mainServer.Run(addr)
	launchBrowser(addr)
	mainServer.Close()
}
