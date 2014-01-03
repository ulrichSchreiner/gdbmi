package gdbmi

import (
	"log"
	"testing"
)

func dummyTokenGenerator() int64 {
	return 1
}

func Test_NewGDB(t *testing.T) {
	tokenGenerator = dummyTokenGenerator
	gdb, err := NewGDB("gdb", "/home/usc/workspaces/gotest/bin/usc2", []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed starting simple process: %s", err)
	}
	//gdb.Break_insert("main", false, false, false, false, false, nil, nil, nil)
	res, err := gdb.Exec_run(false, nil)
	log.Printf("exec result: %+v", res)

	for ev := range gdb.Event {
		if ev.StopReason == Async_stopped_exited ||
			ev.StopReason == Async_stopped_exited_normally ||
			ev.StopReason == Async_stopped_exited_signalled {
			log.Printf("exit received: %+v", ev)
			break
		} else {
			log.Printf("received: %+v", ev)
		}
	}
}
