package gdbmi

import (
	"fmt"
)

var (
	msg  = `{number="1",type="breakpoint",disp="keep",enabled="y",addr="0x00000000004214a0",func="main",file="/usr/local/go/src/pkg/runtime/rt0_linux_amd64.s",fullname="/usr/local/go/src/pkg/runtime/rt0_linux_amd64.s",line="14",thread-groups=["i1"],times="1",original-location="main"}`
	msg2 = `reason="breakpoint-hit",disp="keep",bkptno="1",frame={addr="0x0000000000400d10",func="main.sub",args=[{name="s2",value="..."},{name="s1",value="..."},{name="~anon2",value="..."}],file="/home/usc/workspaces/gdbmi/src/github.com/ulrichSchreiner/gdbmi/cmd/main.go",fullname="/home/usc/workspaces/gdbmi/src/github.com/ulrichSchreiner/gdbmi/cmd/main.go",line="14"},thread-id="1",stopped-threads="all",core="0"`
)

func ExampleStructureParser() {
	g := parseStructure(msg)
	tg := g["thread-groups"].([]interface{})
	fmt.Printf("number=%s,type=%s,disp=%s,enabled=%s,addr=%s,func=%s,file=%s,fullname=%s,times=%s,original-location=%s\n", g["number"], g["type"], g["disp"], g["enabled"], g["addr"], g["func"], g["file"], g["fullname"], g["times"], g["original-location"])
	for i, t := range tg {
		fmt.Printf("%d:%s\n", i, t)
	}
	// Output: number=1,type=breakpoint,disp=keep,enabled=y,addr=0x00000000004214a0,func=main,file=/usr/local/go/src/pkg/runtime/rt0_linux_amd64.s,fullname=/usr/local/go/src/pkg/runtime/rt0_linux_amd64.s,times=1,original-location=main
	// 0:i1
}
