package gdbmi

import (
	"fmt"
	"strconv"
)

type StackListType int

type StackFrame struct {
	Level    int    `json:"level"`
	Function string `json:"function"`
	Address  string `json:"address"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	From     string `json:"from"`
	Fullname string `json:"fullname"`
}

type FrameArgument struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type StackFrameArguments struct {
	Level     int             `json:"level"`
	Arguments []FrameArgument `json:"arguments"`
}

const (
	ListType_no_values StackListType = iota
	ListType_all_values
	ListType_simple_values
)

func stackFrameInfo(sinfo gdbStruct) (*StackFrame, error) {
	var result StackFrame

	fmt.Sscanf(mapValueAsString(sinfo, "line", "0"), "%d", &result.Line)
	fmt.Sscanf(mapValueAsString(sinfo, "level", "0"), "%d", &result.Level)
	result.Function = mapValueAsString(sinfo, "func", "")
	result.Address = mapValueAsString(sinfo, "addr", "")
	result.File = mapValueAsString(sinfo, "file", "")
	result.From = mapValueAsString(sinfo, "from", "")
	result.Fullname = mapValueAsString(sinfo, "fullname", "")

	return &result, nil
}

func parseStackFrameInfo(info string) (*StackFrame, error) {
	return stackFrameInfo(parseStructure(info))
}

func frameArguments(args []interface{}) []FrameArgument {
	var result []FrameArgument
	for _, sa := range args {
		fa := new(FrameArgument)
		samap := sa.(gdbStruct)
		fa.Name = mapValueAsString(samap, "name", "")
		fa.Type = mapValueAsString(samap, "type", "")
		fa.Value = mapValueAsString(samap, "value", "")
		result = append(result, *fa)
	}
	return result
}

func parseStackFrameArguments(info string) (*[]StackFrameArguments, error) {
	var result []StackFrameArguments
	args := parseStructureArray(info)
	for _, arg := range args {
		sf := new(StackFrameArguments)
		sfa := arg.(gdbStruct)
		framemap := sfa["frame"]
		frame := framemap.(gdbStruct)
		fmt.Sscanf(mapValueAsString(frame, "level", "0"), "%d", &sf.Level)
		sf.Arguments = frameArguments(frame["args"].([]interface{}))
		result = append(result, *sf)
	}
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
		finfo := cutoff(res.Results, "frame=", false)
		return parseStackFrameInfo(finfo)
	}
	return nil, err
}

func (gdb *GDB) Stack_info_depth(maxdepth *int) (int, error) {
	c := newCommand("stack-info-depth")
	if maxdepth != nil {
		c.add_param(fmt.Sprintf("%d", *maxdepth))
	}
	res, err := gdb.send(c)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(cutoff(res.Results, "depth=", true))
}

func (gdb *GDB) Stack_list_arguments(lsttype StackListType, lowframe *int, highframe *int) (*[]StackFrameArguments, error) {
	c := newCommand("stack-list-arguments").add_param(fmt.Sprintf("%d", int(lsttype)))
	if lowframe != nil {
		c.add_param(fmt.Sprintf("%d", *lowframe))
	}
	if highframe != nil {
		c.add_param(fmt.Sprintf("%d", *highframe))
	}
	res, err := gdb.send(c)
	if err != nil {
		return nil, err
	}
	data := cutoff(res.Results, "stack-args=", false)
	return parseStackFrameArguments(data)
}
