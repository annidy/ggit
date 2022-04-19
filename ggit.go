package main

import (
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func newCmd(dir string, shell string) error {
	log.Println(shell)
	c := exec.Command("/bin/sh", "-c", shell)
	c.Dir = dir

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH                        // Initial resize.
	defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	// Copy stdin to the pty and the pty to stdout.
	// NOTE: The goroutine will keep reading until the next keystroke before returning.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, ptmx)

	return nil
}

func pull(dir string) {
	execCmd := exec.Command("/bin/sh", "-c", "git stash")
	execCmd.Dir = dir
	stdoutStderr, err := execCmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	noStash := strings.Contains(string(stdoutStderr), "No local changes to save")

	err = newCmd(dir, "git pull")
	if err != nil {
		log.Fatal(dir, ":", err)
	}

	if !noStash {
		execCmd := exec.Command("/bin/sh", "-c", "git stash pop")
		execCmd.Dir = dir
		_, err := execCmd.CombinedOutput()
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
	err := newCmd(dir, "git "+strings.Join(arg, " "))
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
	fi, _ := os.Stat(".")
	files = append(files, fs.FileInfoToDirEntry(fi))

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
