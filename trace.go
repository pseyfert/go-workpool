/*
 * Copyright (C) 2019  CERN for the benefit of the LHCb collaboration
 * Author: Paul Seyfert <pseyfert@cern.ch>
 *
 * This software is distributed under the terms of the GNU General Public
 * Licence version 3 (GPL Version 3), copied verbatim in the file "LICENSE".
 */

package workpool

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"reflect"
	"sync"
	"time"
)

var (
	reftime   time.Time
	sinkMu    sync.Mutex
	sink      io.Writer = ioutil.Discard
	dotracing bool      = false
)

type TraceEvent struct {
	Name           string      `json:"name"` // name of the event, as displayed in Trace Viewer
	Categories     string      `json:"cat"`  // event categories (comma-separated)
	Type           string      `json:"ph"`   // event type (single character)
	ClockTimestamp uint64      `json:"ts"`   // tracing clock timestamp (microsecond granularity)
	Duration       uint64      `json:"dur"`
	Pid            uint64      `json:"pid"` // process ID for the process that output this event
	Tid            uint64      `json:"tid"` // thread ID for the thread that output this event
	Args           interface{} `json:"args"`
}

type namer interface {
	Name() string
}

func makeTraceEvent(output Output, tid uint64) TraceEvent {
	var name string
	if v, ok := output.Cmd.(namer); ok {
		name = v.Name()
	} else if v, ok := output.Cmd.(*exec.Cmd); ok {
		name = v.Args[len(v.Args)-1]
	}
	return TraceEvent{
		Name:           name,
		Type:           "X",
		ClockTimestamp: uint64(output.Start.Sub(reftime) / time.Microsecond),
		Duration:       uint64(output.End.Sub(output.Start) / time.Microsecond),
		Pid:            0,
		Tid:            tid,
	}
}

func traceOutput(output Output, tid uint64) {
	if dotracing {
		writeTraceEvent(makeTraceEvent(output, tid))
	}
}

func startTrace(sink_ io.Writer) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	if sink_ == nil || reflect.ValueOf(sink_).IsNil() {
		dotracing = false
		return
	}
	dotracing = true
	reftime = time.Now()
	sink = sink_
	sink.Write([]byte{'['}) // trailing ] optional
}

func writeTraceEvent(te TraceEvent) {
	b, err := json.Marshal(te)
	if err != nil {
		panic(err)
	}
	sinkMu.Lock()
	defer sinkMu.Unlock()
	if _, err := sink.Write(append(b, ',')); err != nil {
		log.Printf("[trace] %v", err)
	}
}
