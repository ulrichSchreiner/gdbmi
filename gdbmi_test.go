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
	bp, err := gdb.Break_insert("main.go:11", false, false, false, false, false, nil, nil, nil)
	//bp2,  _ : gdb.Break_insert("main.go:10", false, false, false, false, false, nil, nil, nil)
	if err != nil {
		log.Printf("could not insert breakpoint: %s", err)
	} else {
		log.Printf("bp=%+v\n", bp)
		x, _ := gdb.Break_info(bp.Number)
		log.Printf("-->TABLE: %+v", x)
	}
	/*r, err := gdb.Break_after(bp.Number, 0)
	if err != nil {
		log.Printf("could not set break-after: %s", err)
	} else {
		log.Printf("res=%+v\n", r)
	}*/
	//r, err := gdb.Break_commands(bp.Number, "continue")
	//log.Printf("break_commands: %+v, %s", r, err)
	res, err := gdb.Exec_run(false, nil)
	if err != nil {
		log.Printf("exec error: %+s", err)
	} else {
		log.Printf("exec result: %+v", res)
	}
	for ev := range gdb.Event {
		log.Printf("received: %+v", ev)
		if ev.StopReason == Async_stopped_exited ||
			ev.StopReason == Async_stopped_exited_normally ||
			ev.StopReason == Async_stopped_exited_signalled {
			log.Printf("exit received: %+v", ev)
			break
		} else {
			if ev.Type == Async_stopped {
				sf, e := gdb.Stack_info_frame()
				log.Printf("--> %+v:%s\n", sf, e)
				s, e := gdb.Stack_list_locals(ListType_all_values)
				log.Printf("--> %s:%s\n", s, e)
			}
		}
	}
}
