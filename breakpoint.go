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
	c.add_option_when(istemp, "-t")
	c.add_option_when(ishw, "-h")
	c.add_option_when(createpending, "-f")
	c.add_option_when(disabled, "-d")
	c.add_option_when(tracepoint, "-a")
	c.add_option_stringvalue("-c", condition)
	c.add_option_intvalue("-i", ignorecount)
	c.add_option_intvalue("-p", threadid)
	res, err := gdb.send(c)
	if err != nil {
		return nil, err
	}
	if res.Type != Result_done && res.Type != Result_running {
		return nil, fmt.Errorf("breakpoint insertion was not successful:%s", res.Results)
	}
	if strings.HasPrefix(res.Results, "bkpt=") {
		ln := cutoff(res.Results, "bkpt=", false)
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
		parseblock := cutoff(binfo, "bkpt=", false)
		return parseBreakpointInfo(parseblock)
	}
	return nil, nil
}

func (gdb *GDB) Break_list() (*[]Breakpoint, error) {
	var result []Breakpoint
	c := newCommand("break-list")
	r, e := gdb.send(c)
	if e != nil {
		return nil, e
	}
	breakinfo := breakpoint_info.FindAllString(r.Results, -1)
	for _, bi := range breakinfo {
		parseblock := cutoff(bi, "bkpt=", false)
		bp, err := parseBreakpointInfo(parseblock)
		if err != nil {
			return &result, err
		}
		result = append(result, *bp)
	}

	return &result, nil
}

func (gdb *GDB) Break_passcount(number string, count int) (*GDBResult, error) {
	c := newCommand("break-passcount").add_param(number).add_param(fmt.Sprintf("%d", count))
	return gdb.send(c)
}

func (gdb *GDB) Break_watch(expr string, read bool, write bool) (*GDBResult, error) {
	if !(read || write) {
		return nil, nil
	}
	option := ""
	if read && write {
		option = "-a"
	} else if read {
		option = "-r"
	}
	c := newCommand("break-watch").add_option(option)
	return gdb.send(c)
}

func (gdb *GDB) Catch_load(reg string, temp bool, disabled bool) (*GDBResult, error) {
	c := newCommand("catch-load").add_option_when(temp, "-t").add_option_when(disabled, "-d").add_param(reg)
	return gdb.send(c)
}

func (gdb *GDB) Catch_unload(reg string, temp bool, disabled bool) (*GDBResult, error) {
	c := newCommand("catch-unload").add_option_when(temp, "-t").add_option_when(disabled, "-d").add_param(reg)
	return gdb.send(c)
}
