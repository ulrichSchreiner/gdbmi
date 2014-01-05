package gdbmi

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	breakpoint_info = regexp.MustCompile(`bkpt=\{.*?\}`)
)

// Information about a breakpoint.
type Breakpoint struct {
	Number           string
	Type             BreakpointType
	Disposition      BreakpointDispositionType
	Enabled          bool
	Address          string
	Function         string
	Filename         string
	Fullname         string
	Line             int
	At               string
	Pending          string
	Thread           string
	Condition        string
	Ignore           int
	Enable           int
	Mask             string
	Pass             int
	OriginalLocation string
	Times            int
	Installed        bool
	// static-tracepoint-marker-string-id
	// evaluated-by ?
	// catch-type ?
}

func parseBreakpointInfo(info string) (*Breakpoint, error) {
	var result Breakpoint
	binfo := parseStructure(info)
	result.Number = binfo["number"].(string)
	t, ok := BreakpointWithName(binfo["type"].(string))
	if ok {
		result.Type = t
	} else {
		return nil, fmt.Errorf("unknown breakpoint-type: %s", binfo["type"])
	}
	d, ok := BreakpointDispositionWithName(binfo["disp"].(string))
	if ok {
		result.Disposition = d
	} else {
		return nil, fmt.Errorf("unknown breakpoint-disposition-type: %s", binfo["disp"])
	}
	result.Enabled = equals("y", mapValueAsString(binfo, "enabled", "n"))
	result.Address = mapValueAsString(binfo, "addr", "")
	result.Function = mapValueAsString(binfo, "func", "")
	result.Filename = mapValueAsString(binfo, "filename", "")
	result.Fullname = mapValueAsString(binfo, "fullname", "")
	fmt.Sscanf(mapValueAsString(binfo, "line", "0"), "%d", &result.Line)
	result.At = mapValueAsString(binfo, "at", "")
	result.Pending = mapValueAsString(binfo, "pending", "")
	result.Thread = mapValueAsString(binfo, "thread", "")
	result.Condition = mapValueAsString(binfo, "cond", "")
	fmt.Sscanf(mapValueAsString(binfo, "ignore", "0"), "%d", &result.Ignore)
	fmt.Sscanf(mapValueAsString(binfo, "enable", "0"), "%d", &result.Enable)
	result.Mask = mapValueAsString(binfo, "mask", "")
	fmt.Sscanf(mapValueAsString(binfo, "pass", "0"), "%d", &result.Pass)
	result.OriginalLocation = mapValueAsString(binfo, "original-location", "0")
	fmt.Sscanf(mapValueAsString(binfo, "times", "0"), "%d", &result.Times)
	result.Installed = equals("y", mapValueAsString(binfo, "installed", "n"))
	return &result, nil
}

func (gdb *GDB) Break_insert(location string, istemp bool, ishw bool, createpending bool, disabled bool, tracepoint bool, condition *string, ignorecount *int, threadid *int) (*Breakpoint, error) {
	c := newCommand("break-insert").add_param(location)
	if istemp {
		c.add_option("-t")
	}
	if ishw {
		c.add_option("-h")
	}
	if createpending {
		c.add_option("-f")
	}
	if disabled {
		c.add_option("-d")
	}
	if tracepoint {
		c.add_option("-a")
	}
	if condition != nil {
		c.add_optionvalue("-c", *condition)
	}
	if ignorecount != nil {
		c.add_optionvalue("-i", fmt.Sprintf("%d", *ignorecount))
	}
	if threadid != nil {
		c.add_optionvalue("-p", fmt.Sprintf("%d", *threadid))
	}
	res, err := gdb.send(c)
	if err != nil {
		return nil, err
	}
	if res.Type != Result_done && res.Type != Result_running {
		return nil, fmt.Errorf("breakpoint insertion was not successful:%s", res.Results)
	}
	if strings.HasPrefix(res.Results, "bkpt=") {
		ln := string([]byte(res.Results)[len("bkpt="):])
		return parseBreakpointInfo(ln)
	}
	return nil, fmt.Errorf("breakpoint info should start with 'bkpt=', but has value '%s'", res.Results)
}

func (gdb *GDB) Break_after(number string, count int) (*GDBResult, error) {
	c := newCommand("break-after").add_param(number).add_param(fmt.Sprintf("%d", count))
	return gdb.send(c)
}

func (gdb *GDB) Break_commands(number string, cmds ...string) (*GDBResult, error) {
	c := newCommand("break-commands").add_param(number)

	for _, cmd := range cmds {
		c.add_param(fmt.Sprintf("\"%s\"", cmd))
	}
	//c.add_param("end")
	return gdb.send(c)
}

func (gdb *GDB) Break_condition(number string, cond string) (*GDBResult, error) {
	c := newCommand("break-condition").add_param(number).add_param(cond)
	return gdb.send(c)
}

func (gdb *GDB) Break_delete(number ...string) (*GDBResult, error) {
	c := newCommand("break-delete")
	for _, n := range number {
		c.add_param(n)
	}
	return gdb.send(c)
}

func (gdb *GDB) Break_disable(number ...string) (*GDBResult, error) {
	c := newCommand("break-disable")
	for _, n := range number {
		c.add_param(n)
	}
	return gdb.send(c)
}

func (gdb *GDB) Break_enable(number ...string) (*GDBResult, error) {
	c := newCommand("break-enable")
	for _, n := range number {
		c.add_param(n)
	}
	return gdb.send(c)
}

func (gdb *GDB) Break_info(number string) (*Breakpoint, error) {
	c := newCommand("break-info")
	c.add_param(number)
	r, e := gdb.send(c)
	if e != nil {
		return nil, e
	}
	breakinfo := breakpoint_info.FindAllString(r.Results, 1)
	if len(breakinfo) > 0 {
		binfo := breakinfo[0]
		parseblock := string([]byte(binfo)[len("bkpt="):])
		return parseBreakpointInfo(parseblock)
	}
	return nil, nil
}
