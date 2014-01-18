package gdbmi

import (
	"fmt"
	"log"
	_ "net/http"
	_ "net/http/pprof"
	"os"
	"testing"
)

func dummyTokenGenerator() int64 {
	return 1
}

func dummyStart(gdb *GDB, gdbpath string, gdbparms []string, env []string) error {
	return nil
}

func createSender(r *GDBResult, e error) func(cmd *gdb_command) (*GDBResult, error) {
	return func(cmd *gdb_command) (*GDBResult, error) {
		return r, e
	}
}

func TestEnvironment(t *testing.T) {
	gdb := NewGDB("unused")
	gdb.start = dummyStart
	// testfunc for environment-path
	ep := func() (string, error) {
		return gdb.Environment_path(false, "")
	}

	// testfunc for environment-directory
	ed := func() (string, error) {
		return gdb.Environment_directory(false, "")
	}

	pwd := func() (string, error) {
		return gdb.Environment_pwd()
	}

	testdata := []struct {
		result   string
		expected string
		f        func() (string, error)
	}{
		{result: "path=\"/a/b/c\"", expected: "/a/b/c", f: ep},
		{result: "source-path=\"/a/b/c\"", expected: "/a/b/c", f: ed},
		{result: "cwd=\"/a/b/c\"", expected: "/a/b/c", f: pwd},
	}
	for _, td := range testdata {
		res1 := GDBResult{Type: Result_done, Results: td.result, ErrorMessage: ""}
		gdb.send = createSender(&res1, nil)
		envpath, _ := td.f()
		if !equals(envpath, td.expected) {
			t.Errorf("result has wrong value: '%s'", envpath)
			t.Fail()
		}
	}
}

func TestNewGDB(t *testing.T) {
	tokenGenerator = dummyTokenGenerator
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatalf("could not get WorkingDirectory: %s", e)
	}
	gdb := NewGDB("gdb")
	err := gdb.Start(fmt.Sprintf("%s/../../../../bin/cmd", cwd))
	if err != nil {
		t.Fatalf("Failed starting simple process: %s", err)
	}
	//_, err = gdb.Break_insert("main.go:11", false, false, false, false, false, nil, nil, nil)
	//gdb.Break_insert("main.go:15", false, false, false, false, false, nil, nil, nil)
	if err != nil {
		log.Printf("could not insert breakpoint: %s", err)
	} else {
		x, _ := gdb.Break_list()
		log.Printf("Breakpoints: %+v", x)
	}
	/*r, err := gdb.Break_after(bp.Number, 0)
	if err != nil {
		log.Printf("could not set break-after: %s", err)
	} else {
		log.Printf("res=%+v\n", r)
	}*/
	//r, err := gdb.Break_commands(bp.Number, "continue")
	//log.Printf("break_commands: %+v, %s", r, err)
	res, err := gdb.Exec_run(false, false, nil)
	if err != nil {
		log.Printf("exec error: %+s", err)
	} else {
		log.Printf("exec result: %+v", res)
	}
	go func() {
		for ev := range gdb.Event {
			log.Printf("received: %+v", ev)
			if ev.StopReason == Async_stopped_exited ||
				ev.StopReason == Async_stopped_exited_normally ||
				ev.StopReason == Async_stopped_exited_signalled {
				log.Printf("exit received: %+v", ev)
				gdb.Gdb_exit()
				gdb.Close()
				break
			} else {
				if ev.Type == Async_stopped {
					sf, e := gdb.Stack_info_frame()
					log.Printf("--> %+v:%s\n", sf, e)
					s, e := gdb.Stack_list_variables(ListType_all_values)
					log.Printf("--> %v:%s\n", s, e)
					gs, e := gdb.Stack_list_arguments(ListType_all_values, nil, nil)
					log.Printf("--> %+v:%s\n", gs, e)
				}
			}
		}
	}()
	//http.ListenAndServe("localhost:6060", nil)
}
