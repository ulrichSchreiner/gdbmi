package gdbmi

import (
	"fmt"
)

type StackListType int

type StackFrame struct {
	Level    int
	Function string
	Address  string
	File     string
	Line     int
	From     string
}

const (
	ListType_no_values StackListType = iota
	ListType_all_values
	ListType_simple_values
)

func parseStackFrameInfo(info string) (*StackFrame, error) {
	var result StackFrame
	sinfo := parseStructure(info)

	fmt.Sscanf(mapValueAsString(sinfo, "line", "0"), "%d", &result.Line)
	fmt.Sscanf(mapValueAsString(sinfo, "level", "0"), "%d", &result.Level)
	result.Function = mapValueAsString(sinfo, "func", "")
	result.Address = mapValueAsString(sinfo, "addr", "")
	result.File = mapValueAsString(sinfo, "file", "")
	result.From = mapValueAsString(sinfo, "from", "")

	return &result, nil
}

func (gdb *GDB) Stack_list_locals(listtype StackListType) (string, error) {
	return gdb.stack_list("stack-list-locals", listtype)
}
func (gdb *GDB) Stack_list_variables(listtype StackListType) (string, error) {
	return gdb.stack_list("stack-list-variables", listtype)
}
func (gdb *GDB) stack_list(listcommand string, listtype StackListType) (string, error) {
	c := newCommand(listcommand)
	c.add_param(fmt.Sprintf("%d", int(listtype)))
	res, err := gdb.send(c)
	if err == nil {
		return res.Results, err
	}
	return "", err
}
func (gdb *GDB) Stack_info_frame() (*StackFrame, error) {
	c := newCommand("stack-info-frame")
	res, err := gdb.send(c)
	if err == nil {
		finfo := string([]byte(res.Results)[len("frame="):])
		return parseStackFrameInfo(finfo)
	}
	return nil, err

}
