package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
)

func pull(dir string) {
	execCmd := exec.Command("/bin/sh", "-c", "cd '"+dir+"' && git stash")
	stdoutStderr, err := execCmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	noStash := strings.Contains(string(stdoutStderr), "No local changes to save")

	execCmd = exec.Command("/bin/sh", "-c", "cd '"+dir+"' && git pull")
	stdoutStderr, err = execCmd.CombinedOutput()
	fmt.Printf("%s\n", stdoutStderr)
	if err != nil {
		log.Fatal(dir, ":", err)
	}

	if !noStash {
		execCmd := exec.Command("/bin/sh", "-c", "cd '"+dir+"' && git stash pop")
		stdoutStderr, err := execCmd.CombinedOutput()
		fmt.Printf("%s\n", stdoutStderr)
		if err != nil {
			log.Fatal("stash pop:", err)
		}
	}
}

func run(dir string, arg ...string) {
	if len(arg) == 1 && arg[0] == "pull" {
		pull(dir)
		return
	}
	execCmd := exec.Command("/bin/sh", "-c", "cd '"+dir+"' && git "+strings.Join(arg, " "))
	stdoutStderr, err := execCmd.CombinedOutput()
	fmt.Printf("%s\n", stdoutStderr)
	if err != nil {
		log.Fatal(dir, ":", err)
	}
}

func main() {
	files, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) == 1 {
		log.Fatal("need a command")
	}

	var wg sync.WaitGroup

	for _, file := range files {
		if file.IsDir() {
			if _, err := os.Stat(path.Join(file.Name(), ".git")); os.IsNotExist(err) {
				continue
			}
			wg.Add(1)
			go func(dir string) {
				run(dir, os.Args[1:]...)
				wg.Done()
			}(file.Name())
		}
	}
	wg.Wait()
}
