/*
 * Copyright (C) 2019  CERN for the benefit of the LHCb collaboration
 * Author: Paul Seyfert <pseyfert@cern.ch>
 *
 * This software is distributed under the terms of the GNU General Public
 * Licence version 3 (GPL Version 3), copied verbatim in the file "LICENSE".
 */

package main

import (
	"flag"
	"fmt"
	"os/exec"

	"github.com/pseyfert/go-workpool"
)

func prepare(pipe chan *exec.Cmd, i int) {
	targettime := fmt.Sprintf("%ds", i)
	pipe <- exec.Command("sleep", targettime)
}

func main() {
	conc := flag.Int("j", 1, "concurrency")
	N := flag.Int("N", 30, "total number of jobs")
	flag.Parse()

	procpipe, outpipe := workpool.Workpool(*conc)

	go func() {
		for i := 0; i < *N; i += 1 {
			prepare(procpipe, 5+(i%2))
		}
		fmt.Printf("last job submitted\n")
		close(procpipe)
	}()

	workpool.DefaultPrint(outpipe)
	// workpool.DrawProgress(outpipe, *N)
	// workpool.AbortOnFailure(outpipe)
}
