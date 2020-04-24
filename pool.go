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
	"time"

	"github.com/schollz/progressbar"
)

type Runner interface {
	Run() error
}

type SetStdoutStderrer interface {
	SetStdout(io.Writer)
	SetStderr(io.Writer)
}

type Output struct {
	Stderr bytes.Buffer
	Stdout bytes.Buffer
	Err    error
	Start  time.Time
	End    time.Time
	Cmd    Runner
}

func process_pipe(tasks chan Runner, outpipe chan Output) {
	conc := cap(tasks)

	process := func(cmd Runner) Output {
		var out Output
		if v, ok := cmd.(*exec.Cmd); ok {
			v.Stdout = &out.Stdout
			v.Stderr = &out.Stderr
		} else if v, ok := cmd.(SetStdoutStderrer); ok {
			v.SetStdout(&out.Stdout)
			v.SetStderr(&out.Stderr)
		}
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
			for task := range tasks {
				output := process(task)
				traceOutput(output, tid)
				outpipe <- output
			}
			wg.Done()
			return
		}(i)
	}
	wg.Wait()
	close(outpipe)
}

func Workpool(concurrency int, sink io.Writer) (chan Runner, chan Output) {
	startTrace(sink)
	procpipe := make(chan Runner, concurrency)
	outpipe := make(chan Output, concurrency*2) // just allow some more on the output

	go process_pipe(procpipe, outpipe)

	return procpipe, outpipe
}

func errorPrint(out *Output) {
	if exitError, ok := out.Err.(*exec.ExitError); ok {
		if v, ok := out.Cmd.(*exec.Cmd); ok {
			fmt.Printf("%d command failed: %s\n", exitError.ExitCode(), strings.Join(v.Args, " "))
		} else {
			fmt.Printf("%d command failed\n", exitError.ExitCode())
		}
	} else {
		if v, ok := out.Cmd.(*exec.Cmd); ok {
			fmt.Printf("could not run %s: %v\n", v.Path, out.Err)
		} else {
			fmt.Printf("could not run: %v\n", out.Err)
		}
	}
}

func DefaultPrint(outpipe chan Output) {
	for out := range outpipe {
		if out.Err != nil {
			errorPrint(&out)
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

func DrawProgress(outpipe chan Output, length int) {
	bar := progressbar.NewOptions(length, progressbar.OptionShowIts(), progressbar.OptionShowCount(), progressbar.OptionClearOnFinish())
	bar.RenderBlank()
	for out := range outpipe {
		if out.Err != nil {
			fmt.Println()
			errorPrint(&out)
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
	bar.Finish()
}

func AbortOnFailure(outpipe chan Output) {
	for out := range outpipe {
		if out.Err != nil {
			if exitError, ok := out.Err.(*exec.ExitError); ok {
				if v, ok := out.Cmd.(*exec.Cmd); ok {
					fmt.Printf("%d command failed: %s\n", exitError.ExitCode(), strings.Join(v.Args, " "))
				} else {
					fmt.Printf("%d command failed\n", exitError.ExitCode())
				}
			} else {
				fmt.Printf("could not run: %v\n", out.Err)
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
