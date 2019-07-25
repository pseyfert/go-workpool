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
	ack := make(chan bool)

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

	for i := 0; i < conc; i += 1 {
		go func() {
			for {
				task, ok := <-tasks
				if ok {
					outpipe <- process(task)
				} else {
					ack <- true
					return
				}
			}
		}()
	}
	for i := 0; i < conc; i += 1 {
		<-ack
	}
	close(outpipe)
}

func Workpool(concurrency int) (chan *exec.Cmd, chan Output) {
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
					fmt.Printf("%d command failed: %s %s\n", exitError.ExitCode(), out.Cmd.Path, strings.Join(out.Cmd.Args, " "))
				} else {
					fmt.Printf("could not run %s: %v\n", out.Cmd.Path, out.Err)
				}
			}
			io.Copy(os.Stdout, &out.Stdout)
			io.Copy(os.Stderr, &out.Stderr)
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
					fmt.Printf("%d command failed: %s %s\n", exitError.ExitCode(), out.Cmd.Path, strings.Join(out.Cmd.Args, " "))
				} else {
					fmt.Printf("could not run %s: %v\n", out.Cmd.Path, out.Err)
				}
			}
			// TODO: inser line break if not done already, but only if there is some printout
			io.Copy(os.Stdout, &out.Stdout)
			io.Copy(os.Stderr, &out.Stderr)
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
					fmt.Printf("%d command failed: %s %s\n", exitError.ExitCode(), out.Cmd.Path, strings.Join(out.Cmd.Args, " "))
				} else {
					fmt.Printf("could not run %s: %v\n", out.Cmd.Path, out.Err)
				}
			}
			io.Copy(os.Stdout, &out.Stdout)
			io.Copy(os.Stderr, &out.Stderr)
			if out.Err != nil {
				break
			}
		}
	}
}
