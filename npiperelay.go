// Package main contains the npiperelay application
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// How long to sleep between failures while polling
const (
	cSECURITY_SQOS_PRESENT = 0x100000               //nolint:revive,stylecheck // Don't include revive and stylecheck when running golangci-lint to stop complain about use of underscores in Go names
	cSECURITY_ANONYMOUS    = 0                      //nolint:revive,stylecheck // Don't include revive and stylecheck when running golangci-lint to stop complain about use of underscores in Go names
	cPOLL_TIMEOUT          = 200 * time.Millisecond //nolint:revive,stylecheck // Don't include revive and stylecheck when running golangci-lint to stop complain about use of underscores in Go names
)

var (
	poll            = flag.Bool("p", false, "poll until the the named pipe exists and is not busy")
	closeWrite      = flag.Bool("s", false, "send a 0-byte message to the pipe after EOF on stdin")
	closeOnEOF      = flag.Bool("ep", false, "terminate on EOF reading from the pipe, even if there is more data to write")
	closeOnStdinEOF = flag.Bool("ei", false, "terminate on EOF reading from stdin, even if there is more data to write")
	runInBackground = flag.Bool("bg", false, "hide console window and run in background")
	verbose         = flag.Bool("v", false, "verbose output on stderr")

	version = "0.0.0-dev" // Replaced with value from ldflag in build by GoReleaser: Current Git tag with the v prefix stripped
	commit  = "unknown"   // Replaced with value from ldflag in build by GoReleaser: Current git commit SHA
	date    = "unknown"   // Replaced with value from ldflag in build by GoReleaser: Date according RFC3339
	builtBy = "unknown"   // Replaced with value from ldflag in build by GoReleaser
)

func hideConsole() error {
	getConsoleWindow := windows.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")
	if err := getConsoleWindow.Find(); err != nil {
		return err
	}
	showWindow := windows.NewLazyDLL("user32.dll").NewProc("ShowWindow")
	if err := showWindow.Find(); err != nil {
		return err
	}
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return errors.New("no console associated")
	}
	iret, _, _ := showWindow.Call(hwnd, 0) // SW_HIDE (0): Hides the window and activates another window.
	if iret == 0 {
		return errors.New("console window is already hidden")
	}
	return nil
}

func dialPipe(p string, poll bool) (*overlappedFile, error) {
	p16, err := windows.UTF16FromString(p)
	if err != nil {
		return nil, err
	}
	for {
		h, err := windows.CreateFile(&p16[0], windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED|cSECURITY_SQOS_PRESENT|cSECURITY_ANONYMOUS, 0)
		if err == nil {
			return newOverlappedFile(h), nil
		}
		if poll {
			if err == windows.ERROR_FILE_NOT_FOUND {
				time.Sleep(cPOLL_TIMEOUT)
				continue
			}
			if err == windows.ERROR_PIPE_BUSY {
				time.Sleep(cPOLL_TIMEOUT)
				continue
			}
		}
		return nil, &os.PathError{Path: p, Op: "open", Err: err}
	}
}

func main() {
	flag.Usage = func() {
		// Custom usage message (default documented here: https://pkg.go.dev/flag#pkg-variables)
		fmt.Fprintf(flag.CommandLine.Output(), "npiperelay v%s\n", version)
		fmt.Fprintf(flag.CommandLine.Output(), "  commit %s\n", commit)
		fmt.Fprintf(flag.CommandLine.Output(), "  build date %s\n", date)
		fmt.Fprintf(flag.CommandLine.Output(), "  built by %s\n", builtBy)
		fmt.Fprintf(flag.CommandLine.Output(), "  built with %s\n", runtime.Version())
		fmt.Fprint(flag.CommandLine.Output(), "\nusage:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	if *runInBackground {
		if err := hideConsole(); err != nil {
			log.Println("hide console window to run in background failed:", err)
		}
	}

	if *verbose {
		log.Println("connecting to", args[0])
	}

	conn, err := dialPipe(args[0], *poll)
	if err != nil {
		log.Fatalln(err)
	}

	if *verbose {
		log.Println("connected")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if _, err := io.Copy(conn, os.Stdin); err != nil {
			log.Fatalln("copy from stdin to pipe failed:", err)
		}

		if *verbose {
			log.Println("copy from stdin to pipe finished")
		}

		if *closeOnStdinEOF {
			os.Exit(0)
		}

		if *closeWrite {
			// A zero-byte write on a message pipe indicates that no more data is coming.
			if _, err := conn.Write(nil); err != nil {
				log.Println("sending 0-byte message to the pipe after EOF on stdin failed:", err)
			}
		}
		if err := os.Stdin.Close(); err != nil {
			log.Println("closing stdin failed:", err)
		}
		wg.Done()
	}()

	_, err = io.Copy(os.Stdout, conn)
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) || errors.Is(err, windows.ERROR_PIPE_NOT_CONNECTED) {
		// The named pipe is closed and there is no more data to read. Since
		// named pipes are not bidirectional, there is no way for the other side
		// of the pipe to get more data, so do not wait for the stdin copy to
		// finish.
		if *verbose {
			log.Println("copy from pipe to stdout finished: pipe closed")
		}
		os.Exit(0)
	}

	if err != nil {
		log.Fatalln("copy from pipe to stdout failed:", err)
	}

	if *verbose {
		log.Println("copy from pipe to stdout finished")
	}

	if !*closeOnEOF {
		if err := os.Stdout.Close(); err != nil {
			log.Println("closing stdout failed:", err)
		}

		// Keep reading until we get ERROR_BROKEN_PIPE or the copy from stdin finishes.
		go func() {
			for {
				_, err := conn.Read(nil)
				if errors.Is(err, windows.ERROR_BROKEN_PIPE) {
					if *verbose {
						log.Println("pipe closed")
					}
					os.Exit(0)
				} else if err != nil {
					log.Fatalln("pipe error:", err)
				}
			}
		}()

		wg.Wait()
	}
}
