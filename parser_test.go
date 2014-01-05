package gdbmi

import (
	"fmt"
)

var (
	msg = `{number="1",type="breakpoint",disp="keep",enabled="y",addr="0x00000000004214a0",func="main",file="/usr/local/go/src/pkg/runtime/rt0_linux_amd64.s",fullname="/usr/local/go/src/pkg/runtime/rt0_linux_amd64.s",line="14",thread-groups=["i1"],times="1",original-location="main"}`
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
