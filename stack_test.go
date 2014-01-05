package gdbmi

import (
	"fmt"
)

const (
	stackinfo1 = `{level="1",addr="0x0001076c",func="callee3",
     file="basics.c",
     fullname="/asdfasdf/basics.c",line="17"}`
)

func ExampleStackInfoParser() {
	g, _ := parseStackFrameInfo(stackinfo1)
	fmt.Printf("level=%d,addr=%s,func=%s,file=%s,line=%d", g.Level, g.Address, g.Function, g.File, g.Line)
	// Output: level=1,addr=0x0001076c,func=callee3,file=basics.c,line=17
}
