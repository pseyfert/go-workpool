/*
 * Copyright (C) 2019  CERN for the benefit of the LHCb collaboration
 * Author: Paul Seyfert <pseyfert@cern.ch>
 *
 * This software is distributed under the terms of the GNU General Public
 * Licence version 3 (GPL Version 3), copied verbatim in the file "LICENSE".
 */

package workpool

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/schollz/progressbar"
)

type Output struct {
	Stderr bytes.Buffer
	Stdout bytes.Buffer
	Err    error
	Start  time.Time
	End    time.Time
	Cmd    *exec.Cmd
}

func process_pipe(tasks chan *exec.Cmd, outpipe chan Output) {
	conc := cap(tasks)

	process := func(cmd *exec.Cmd) Output {
		var out Output
		cmd.Stdout = &out.Stdout
		cmd.Stderr = &out.Stderr
		out.Start = time.Now()
		out.Err = cmd.Run()
		out.End = time.Now()
		out.Cmd = cmd
		return out
	}

	var wg sync.WaitGroup
	wg.Add(conc)
	for i := uint64(0); i < uint64(conc); i += 1 {
		go func(tid uint64) {
			for {
				task, ok := <-tasks
				if ok {
					output := process(task)
					traceOutput(output, tid)
					outpipe <- output
				} else {
					wg.Done()
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(outpipe)
}

func Workpool(concurrency int, sink io.Writer) (chan *exec.Cmd, chan Output) {
	startTrace(sink)
	procpipe := make(chan *exec.Cmd, concurrency)
	outpipe := make(chan Output, concurrency*2) // just allow some more on the output

	go process_pipe(procpipe, outpipe)

	return procpipe, outpipe
}

func DefaultPrint(outpipe chan Output) {
	for {
		out, ok := <-outpipe
		if !ok {
			break
		} else {
			if out.Err != nil {
				if exitError, ok := out.Err.(*exec.ExitError); ok {
					waitStatus := exitError.Sys().(syscall.WaitStatus)
					ec := waitStatus.ExitStatus()
					// ec := exitError.ExitCode()
					fmt.Printf("%d command failed: %s\n", ec, strings.Join(out.Cmd.Args, " "))
				} else {
					fmt.Printf("could not run %s: %v\n", out.Cmd.Path, out.Err)
				}
			}
			_, err := io.Copy(os.Stdout, &out.Stdout)
			if err != nil {
				fmt.Printf("error during output redirection: %v\n", err)
			}
			_, err = io.Copy(os.Stderr, &out.Stderr)
			if err != nil {
				fmt.Printf("error during output redirection: %v\n", err)
			}
		}
	}
}

func DrawProgress(outpipe chan Output, length int) {
	bar := progressbar.NewOptions(length, progressbar.OptionShowIts(), progressbar.OptionShowCount(), progressbar.OptionClearOnFinish())
	bar.RenderBlank()
	for {
		out, ok := <-outpipe
		if !ok {
			break
		} else {
			if out.Err != nil {
				fmt.Println()
				if exitError, ok := out.Err.(*exec.ExitError); ok {
					waitStatus := exitError.Sys().(syscall.WaitStatus)
					ec := waitStatus.ExitStatus()
					// ec := exitError.ExitCode()
					fmt.Printf("%d command failed: %s\n", ec, strings.Join(out.Cmd.Args, " "))
				} else {
					fmt.Printf("could not run %s: %v\n", out.Cmd.Path, out.Err)
				}
			} else if out.Stdout.Len()+out.Stderr.Len() > 0 {
				// insert line break if not done already, but only if there is some printout
				fmt.Println()
			}
			_, err := io.Copy(os.Stdout, &out.Stdout)
			if err != nil {
				fmt.Printf("error during output redirection: %v\n", err)
			}
			_, err = io.Copy(os.Stderr, &out.Stderr)
			if err != nil {
				fmt.Printf("error during output redirection: %v\n", err)
			}
			bar.Add(1)
		}
	}
	bar.Finish()
}

func AbortOnFailure(outpipe chan Output) {
	for {
		out, ok := <-outpipe
		if !ok {
			break
		} else {
			if out.Err != nil {
				if exitError, ok := out.Err.(*exec.ExitError); ok {
					waitStatus := exitError.Sys().(syscall.WaitStatus)
					ec := waitStatus.ExitStatus()
					// ec := exitError.ExitCode()
					fmt.Printf("%d command failed: %s\n", ec, strings.Join(out.Cmd.Args, " "))
				} else {
					fmt.Printf("could not run %s: %v\n", out.Cmd.Path, out.Err)
				}
			}
			_, err := io.Copy(os.Stdout, &out.Stdout)
			if err != nil {
				fmt.Printf("error during output redirection: %v\n", err)
			}
			_, err = io.Copy(os.Stderr, &out.Stderr)
			if err != nil {
				fmt.Printf("error during output redirection: %v\n", err)
			}
			if out.Err != nil {
				break
			}
		}
	}
}
