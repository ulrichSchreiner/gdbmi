package gdbmi

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type creator func() gdb_response

type gdb_output struct {
	*regexp.Regexp
	creator
}

func (p gdb_output) Match(s string) bool {
	return p.MatchString(s)
}

func (p gdb_output) fields(line string) map[string]string {
	res := make(map[string]string)
	names := p.SubexpNames()
	for i := 1; i < len(names); i++ {
		res[names[i]] = p.ReplaceAllString(line, fmt.Sprintf("${%s}", names[i]))
	}
	return res
}

func (p gdb_output) Create(ln string) gdb_response {
	r := p.creator()
	r.Fill(p.fields(ln))
	return r
}

var (
	_gdb_delim                        = []byte("(gdb)")
	tokenGenerator tokenGeneratorType = timetokenGenerator
	result_record                     = gdb_output{
		regexp.MustCompile(`^(?P<token>\d*)\^(?P<message>.*)`),
		func() gdb_response { return new(gdb_result) }}
	console_output = gdb_output{
		regexp.MustCompile(`^~(?P<message>.*)`),
		func() gdb_response { return new(gdb_console_output) }}
	target_output = gdb_output{
		regexp.MustCompile(`^@(?P<message>.*)`),
		func() gdb_response { return new(gdb_target_output) }}
	log_output = gdb_output{
		regexp.MustCompile(`^\&(?P<message>.*)`),
		func() gdb_response { return new(gdb_log_output) }}
	notify_async_output = gdb_output{
		regexp.MustCompile(`^(?P<token>\d*)=(?P<message>.*)`),
		func() gdb_response { return new(gdb_async) }}
	exec_async_output = gdb_output{
		regexp.MustCompile(`^(?P<token>\d*)\*(?P<message>.*)`),
		func() gdb_response { return new(gdb_async) }}
	status_async_output = gdb_output{
		regexp.MustCompile(`^(?P<token>\d*)\+(?P<message>.*)`),
		func() gdb_response { return new(gdb_async) }}
	gdb_delim = gdb_output{
		regexp.MustCompile(`\(gdb\)`),
		func() gdb_response { return new(gdb_ready) }}

	gdb_responses = []gdb_output{
		result_record,
		console_output,
		target_output,
		log_output,
		notify_async_output,
		exec_async_output,
		status_async_output,
	}

	async_running_line = regexp.MustCompile("running,thread-id=\"(.*)\"")
)

type tokenGeneratorType func() int64

type gdb_command struct {
	token     int64
	cmd       string
	parameter []string
	options   []string
	result    chan gdb_response
}

type gdb_response interface {
	Token() int64
	Line() string
	Fill(fields map[string]string)
}
type gdb_response_type struct {
	token int64
	line  string
}
type gdb_ready struct {
	gdb_response_type
}
type gdb_result struct {
	gdb_response_type
}
type gdb_async struct {
	gdb_response_type
}
type gdb_console_output struct {
	gdb_response_type
}
type gdb_log_output struct {
	gdb_response_type
}
type gdb_target_output struct {
	gdb_response_type
}

func (r *gdb_response_type) Token() int64 {
	return r.token
}
func (r *gdb_response_type) Line() string {
	return r.line
}
func (r *gdb_response_type) Fill(fields map[string]string) {
	token, ok := fields["token"]
	if ok && len(token) > 0 {
		tok, perr := strconv.ParseInt(token, 10, 64)
		if perr != nil {
			log.Printf("cannot parse token: %s", perr)
		} else {
			r.token = tok
		}
	}
	r.line = fields["message"]
}

func timetokenGenerator() int64 {
	return time.Now().UnixNano()
}

func newCommand(cmd string) *gdb_command {
	c := new(gdb_command)
	c.token = tokenGenerator()
	c.cmd = cmd
	c.result = make(chan gdb_response)

	return c
}

func (c *gdb_command) add_param(p string) *gdb_command {
	c.parameter = append(c.parameter, p)
	return c
}

func (c *gdb_command) add_optionvalue(opt string, optparam string) *gdb_command {
	c.options = append(c.options, fmt.Sprintf("-%s %s", opt, optparam))
	return c
}

func (c *gdb_command) add_option(opt string) *gdb_command {
	c.options = append(c.options, fmt.Sprintf("-%s", opt))
	return c
}

func (c *gdb_command) dump_mi() string {
	p := strings.Join(c.parameter, " ")
	o := strings.Join(c.options, " ")

	return fmt.Sprintf("%d-%s %s %s", c.token, c.cmd, o, p)
}

type GDBResultType int

const (
	Result_done GDBResultType = iota
	Result_running
	Result_connected
	Result_error
	Result_exit
)

type GDBResult struct {
	Type         GDBResultType
	Results      string
	ErrorMessage string
}

type GDBAsyncType int
type GDBStopReason int
type BreakpointType int
type BreakpointDispositionType int

const (
	BP_breakpoint BreakpointType = iota
	BP_catchpoint

	BP_breakpointDisposition_delete = iota
	BP_breakpointDisposition_keep

	Async_unknown GDBAsyncType = iota
	Async_running
	Async_stopped
	Async_thread_group_added
	Async_thread_group_removed
	Async_thread_group_started
	Async_thread_group_exited
	Async_thread_created
	Async_thread_exited
	Async_thread_selected
	Async_library_loaded
	Async_library_unloaded
	Async_traceframe_changed
	Async_tsv_created
	Async_tsv_deleted
	Async_tsv_modified
	Async_breakpoint_created
	Async_breakpoint_modified
	Async_breakpoint_deleted
	Async_record_started
	Async_record_stopped
	Async_cmd_param_changed
	Async_memory_changed

	Async_stopped_breakpoint_hit GDBStopReason = iota
	Async_stopped_watchpoint_trigger
	Async_stopped_read_watchpoint_trigger
	Async_stopped_access_watchpoint_trigger
	Async_stopped_function_finished
	Async_stopped_location_reached
	Async_stopped_watchpoint_scope
	Async_stopped_end_stepping_range
	Async_stopped_exited_signalled
	Async_stopped_exited
	Async_stopped_exited_normally
	Async_stopped_signal_received
	Async_stopped_solib_event
	Async_stopped_fork
	Async_stopped_vfork
	Async_stopped_syscall_entry
	Async_stopped_exec
)

type StopReasons struct {
	stopReason2Id map[string]GDBStopReason
	stopId2Reason map[GDBStopReason]string
}

func (st *StopReasons) add(name string, r GDBStopReason) {
	st.stopId2Reason[r] = name
	st.stopReason2Id[name] = r
}
func NewStopReasons() *StopReasons {
	var sr StopReasons
	sr.stopReason2Id = make(map[string]GDBStopReason)
	sr.stopId2Reason = make(map[GDBStopReason]string)
	return &sr
}

func StopReasonWithName(n string) (GDBStopReason, bool) {
	sr, ok := stopReasons.stopReason2Id[n]
	return sr, ok
}

type AsyncTypes struct {
	asyncName2TypeId map[string]GDBAsyncType
	asyncTypeId2Name map[GDBAsyncType]string
}

func (st *AsyncTypes) add(name string, r GDBAsyncType) {
	st.asyncTypeId2Name[r] = name
	st.asyncName2TypeId[name] = r
}
func NewAsyncTypes() *AsyncTypes {
	var at AsyncTypes
	at.asyncName2TypeId = make(map[string]GDBAsyncType)
	at.asyncTypeId2Name = make(map[GDBAsyncType]string)
	return &at
}

func AsyncTypeWithName(n string) (GDBAsyncType, bool) {
	at, ok := asyncTypes.asyncName2TypeId[n]
	return at, ok
}

type ResultTypes struct {
	resultId2Type map[GDBResultType]string
	resultType2Id map[string]GDBResultType
}

func (rt *ResultTypes) add(name string, r GDBResultType) {
	rt.resultId2Type[r] = name
	rt.resultType2Id[name] = r
}
func NewResultTypes() *ResultTypes {
	var rt ResultTypes
	rt.resultType2Id = make(map[string]GDBResultType)
	rt.resultId2Type = make(map[GDBResultType]string)
	return &rt
}

func ResultTypeWithName(n string) (GDBResultType, bool) {
	rt, ok := resultTypes.resultType2Id[n]
	return rt, ok
}

type BreakpointTypes struct {
	breakId2Name map[BreakpointType]string
	breakName2Id map[string]BreakpointType
}

func NewBreakpointTypes() *BreakpointTypes {
	var bp BreakpointTypes
	bp.breakId2Name = make(map[BreakpointType]string)
	bp.breakName2Id = make(map[string]BreakpointType)
	return &bp
}
func (bp *BreakpointTypes) add(name string, r BreakpointType) {
	bp.breakId2Name[r] = name
	bp.breakName2Id[name] = r
}
func BreakpointWithName(n string) (BreakpointType, bool) {
	bp, ok := breakpointTypes.breakName2Id[n]
	return bp, ok
}

type BreakpointDispositionTypes struct {
	breakId2Name map[BreakpointDispositionType]string
	breakName2Id map[string]BreakpointDispositionType
}

func NewBreakpointDispositionTypes() *BreakpointDispositionTypes {
	var bp BreakpointDispositionTypes
	bp.breakId2Name = make(map[BreakpointDispositionType]string)
	bp.breakName2Id = make(map[string]BreakpointDispositionType)
	return &bp
}
func (bp *BreakpointDispositionTypes) add(name string, r BreakpointDispositionType) {
	bp.breakId2Name[r] = name
	bp.breakName2Id[name] = r
}
func BreakpointDispositionWithName(n string) (BreakpointDispositionType, bool) {
	bp, ok := breakpointDispositionTypes.breakName2Id[n]
	return bp, ok
}

var (
	stopReasons                = NewStopReasons()
	asyncTypes                 = NewAsyncTypes()
	resultTypes                = NewResultTypes()
	breakpointTypes            = NewBreakpointTypes()
	breakpointDispositionTypes = NewBreakpointDispositionTypes()
)

func init() {

	breakpointTypes.add("breakpoint", BP_breakpoint)
	breakpointTypes.add("catchpoint", BP_catchpoint)

	resultTypes.add("done", Result_done)
	resultTypes.add("running", Result_running)
	resultTypes.add("connected", Result_connected)
	resultTypes.add("error", Result_error)
	resultTypes.add("exit", Result_exit)

	asyncTypes.add("running", Async_running)
	asyncTypes.add("stopped", Async_stopped)
	asyncTypes.add("thread-group-added", Async_thread_group_added)
	asyncTypes.add("thread-group-removed", Async_thread_group_removed)
	asyncTypes.add("thread-group-started", Async_thread_group_started)
	asyncTypes.add("thread-group-exited", Async_thread_group_exited)
	asyncTypes.add("thread-created", Async_thread_created)
	asyncTypes.add("thread-exited", Async_thread_exited)
	asyncTypes.add("thread-selected", Async_thread_selected)
	asyncTypes.add("library-loaded", Async_library_loaded)
	asyncTypes.add("library-unloaded", Async_library_unloaded)
	asyncTypes.add("traceframe-changed", Async_traceframe_changed)
	asyncTypes.add("tsv-created", Async_tsv_created)
	asyncTypes.add("tsv-deleted", Async_tsv_deleted)
	asyncTypes.add("tsv-modified", Async_tsv_modified)
	asyncTypes.add("breakpoint-created", Async_breakpoint_created)
	asyncTypes.add("breakpoint-modified", Async_breakpoint_modified)
	asyncTypes.add("breakpoint-deleted", Async_breakpoint_deleted)
	asyncTypes.add("record-started", Async_record_started)
	asyncTypes.add("record-stopped", Async_record_stopped)
	asyncTypes.add("cmd-param-changed", Async_cmd_param_changed)
	asyncTypes.add("memory-changed", Async_memory_changed)

	stopReasons.add("breakpoint-hit", Async_stopped_breakpoint_hit)
	stopReasons.add("breakpoint-hit", Async_stopped_breakpoint_hit)
	stopReasons.add("watchpoint-trigger", Async_stopped_watchpoint_trigger)
	stopReasons.add("read-watchpoint-trigger", Async_stopped_read_watchpoint_trigger)
	stopReasons.add("access-watchpoint-trigger", Async_stopped_access_watchpoint_trigger)
	stopReasons.add("function-finished", Async_stopped_function_finished)
	stopReasons.add("location-reached", Async_stopped_location_reached)
	stopReasons.add("watchpoint-scope", Async_stopped_watchpoint_scope)
	stopReasons.add("end-stepping-range", Async_stopped_end_stepping_range)
	stopReasons.add("exited-signalled", Async_stopped_exited_signalled)
	stopReasons.add("exited", Async_stopped_exited)
	stopReasons.add("exited-normally", Async_stopped_exited_normally)
	stopReasons.add("signal-received", Async_stopped_signal_received)
	stopReasons.add("solib-event", Async_stopped_solib_event)
	stopReasons.add("fork", Async_stopped_fork)
	stopReasons.add("vfork", Async_stopped_vfork)
	stopReasons.add("syscall-entry", Async_stopped_syscall_entry)
	stopReasons.add("exec", Async_stopped_exec)
}

func (rt GDBResultType) String() string {
	return resultTypes.resultId2Type[rt]
}

func (sr GDBStopReason) String() string {
	return stopReasons.stopId2Reason[sr]
}

func (at GDBAsyncType) String() string {
	return asyncTypes.asyncTypeId2Name[at]
}

func (bp BreakpointType) String() string {
	return breakpointTypes.breakId2Name[bp]
}

func (bp BreakpointDispositionType) String() string {
	return breakpointDispositionTypes.breakId2Name[bp]
}

type GDBEvent struct {
	Type             GDBAsyncType
	StopReason       GDBStopReason
	ThreadId         string
	ThreadGroupid    string
	StoppedThreads   string
	StopeCore        string
	Pid              int
	ExitCode         int
	TraceFrameNumber int
	TracePointNumber int
	TsvName          string
	TsvValue         string
	TsvInitial       string
	CmdParam         string
	CmdValue         string
	MemoryAddress    int
	MemoryLen        int
	MemoryTypeCode   bool
	BreakpointNumber string
}

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

// A running debugger
type GDB struct {
	Event    chan GDBEvent
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	stdin    io.WriteCloser
	commands chan gdb_command
	result   chan gdb_response
}

func NewGDB(gdbpath string, executable string, params []string, env []string) (*GDB, error) {
	gdb := new(GDB)
	gdb.Event = make(chan GDBEvent)
	gdb.commands = make(chan gdb_command)
	gdb.result = make(chan gdb_response)
	gdbargs := []string{"-q", "-i", "mi"}
	gdbargs = append(gdbargs, executable)
	cmd := exec.Command(gdbpath, gdbargs...)
	pipe, err := cmd.StdoutPipe()

	if err != nil {
		return nil, err
	}
	gdb.stdout = pipe
	go gdb.parse_gdb_output()

	pipe, err = cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	gdb.stderr = pipe
	ipipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	gdb.stdin = ipipe
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		open_commands := make(map[int64]*gdb_command)
		for {
			select {
			case c := <-gdb.commands:
				//log.Printf("received command '%+v':%s\n", c, c.dump_mi())
				gdb.send_to_gdb(&c)
				open_commands[c.token] = &c
			case r := <-gdb.result:
				//log.Printf("received result '%+v'", r)
				switch r.(type) {
				case *gdb_result:
					waiting_cmd, ok := open_commands[r.Token()]
					if ok {
						waiting_cmd.result <- r
					}
				case *gdb_console_output:
					log.Printf("CONSOLE: %+v", r)
				case *gdb_log_output:
					log.Printf("LOG: %+v", r)
				case *gdb_async:
					ev, err := createAsync(r.(*gdb_async))
					if err != nil {
						log.Printf("Async Event Error: %s", err)
					} else {
						go func() {
							gdb.Event <- *ev
						}()
					}
				}
			}
		}
	}()
	return gdb, nil
}

func (gdb *GDB) parse_gdb_output() {
	buf := bufio.NewReader(gdb.stdout)
	for {
		var ln []byte
		ln, _ = buf.ReadBytes('\n')
		ln = bytes.TrimSpace(ln)
		sline := string(ln)
		//log.Printf(" ---> %s", sline)
		if gdb_delim.Match(sline) {
			continue
		} else {
			found := false
			for _, rt := range gdb_responses {
				if rt.Match(sline) {
					found = true
					rsp := rt.Create(sline)
					gdb.result <- rsp
				}
			}
			if !found {
				log.Printf("No parser found for line '%s'", sline)
			}
		}

	}
}

func (gdb *GDB) send_to_gdb(cmd *gdb_command) {
	fmt.Fprintln(gdb.stdin, cmd.dump_mi())
}

func (gdb *GDB) send(cmd *gdb_command) gdb_response {
	gdb.commands <- *cmd
	return <-cmd.result
}

func splitKVList(kvlist string) map[string]string {
	res := make(map[string]string)
	parts := strings.Split(kvlist, ",")
	for _, p := range parts {
		kv := strings.Split(p, "=")
		val := string([]byte(kv[1])[1 : len(kv[1])-1])
		res[kv[0]] = val
	}
	return res
}

func asyncTypeFromString(tp string) GDBAsyncType {
	t, ok := AsyncTypeWithName(tp)
	if !ok {
		return Async_unknown
	}
	return t
}

func createAsync(res *gdb_async) (*GDBEvent, error) {
	var result GDBEvent
	toks := strings.SplitN(res.Line(), ",", 2)
	result.Type = asyncTypeFromString(toks[0])
	sub := []byte(res.Line())[len(result.Type.String())+1:]
	params := splitKVList(string(sub))
	switch result.Type {
	case Async_running:
		result.ThreadId = async_running_line.ReplaceAllString(res.Line(), "$1")
		return &result, nil
	case Async_stopped:
		result.ThreadId, _ = params["thread-id"]
		result.StoppedThreads, _ = params["stopped-threads"]
		result.StopeCore, _ = params["core"]
		reason, _ := params["reason"]
		sr, ok := StopReasonWithName(reason)
		if !ok {
			log.Printf("Error: unknown stopreaseon: %s\n", reason)
		} else {
			result.StopReason = sr
		}
		return &result, nil
	case Async_thread_group_started:
		result.ThreadGroupid, _ = params["id"]
		fmt.Sscanf(params["pid"], "%d", &result.Pid)
	case Async_thread_group_exited:
		result.ThreadGroupid, _ = params["id"]
		fmt.Sscanf(params["exit-code"], "%d", &result.ExitCode)
	case Async_thread_exited, Async_thread_created, Async_thread_selected:
		result.ThreadId, _ = params["id"]
		result.ThreadGroupid, _ = params["gid"]
	case Async_thread_group_added, Async_thread_group_removed:
		result.ThreadGroupid, _ = params["id"]
	case Async_library_loaded, Async_library_unloaded:
		break
	case Async_traceframe_changed:
		fmt.Sscanf(params["num"], "%d", &result.TraceFrameNumber)
		fmt.Sscanf(params["tracepoint"], "%d", &result.TracePointNumber)
	case Async_tsv_created, Async_tsv_deleted, Async_tsv_modified:
		result.TsvName, _ = params["name"]
		result.TsvInitial, _ = params["initial"]
		result.TsvValue, _ = params["current"]
	case Async_record_started, Async_record_stopped:
		result.ThreadGroupid, _ = params["thread-group"]
	case Async_cmd_param_changed:
		result.CmdParam, _ = params["param"]
		result.CmdValue, _ = params["value"]
	case Async_memory_changed:
		result.ThreadGroupid, _ = params["thread-group"]
		fmt.Sscanf(params["addr"], "%d", result.MemoryAddress)
		fmt.Sscanf(params["len"], "%d", result.MemoryLen)
		_, result.MemoryTypeCode = params["type"]
	default:
		return nil, fmt.Errorf("unknown async message: %s", res.Line())
	}
	return &result, nil
}

func createResult(res *gdb_result) (*GDBResult, error) {
	var result GDBResult
	if strings.HasPrefix(res.Line(), Result_done.String()) {
		parts := strings.SplitN(res.Line(), ",", 2)
		if len(parts) > 1 {
			result.Results = parts[1]
		}
		result.Type = Result_done
		return &result, nil
	} else if strings.HasPrefix(res.Line(), string(Result_running)) {
		result.Type = Result_running
		return &result, nil
	} else if strings.HasPrefix(res.Line(), string(Result_connected)) {
		result.Type = Result_connected
		return &result, nil
	} else if strings.HasPrefix(res.Line(), string(Result_error)) {
		parts := strings.SplitN(res.Line(), ",", 2)
		if len(parts) > 1 {
			result.ErrorMessage = parts[1]
		}
		result.Type = Result_error
		return &result, nil
	} else if strings.HasPrefix(res.Line(), string(Result_exit)) {
		result.Type = Result_exit
		return &result, nil
	}
	return nil, fmt.Errorf("unknown result indication '%s'", res.Line())
}

func (gdb *GDB) Exec_next() {
	c := newCommand("exec-next")
	res := gdb.send(c)
	log.Printf("Result of exec-next: %+v", res)
}

func (gdb *GDB) Exec_nexti(reverse bool) {
	c := newCommand("exec-next-instruction")
	if reverse {
		c.add_option("--reverse")
	}
	res := gdb.send(c)
	log.Printf("Result of exec-nexti: %+v", res)
}

func (gdb *GDB) Exec_run(all bool, threadgroup *int) (*GDBResult, error) {
	c := newCommand("exec-run")
	if all {
		c.add_option("--all")
	}
	if threadgroup != nil {
		c.add_optionvalue("--thread-group", fmt.Sprintf("%d", *threadgroup))
	}
	res := gdb.send(c)
	return createResult(res.(*gdb_result))
}

func (gdb *GDB) Break_insert(location string, istemp bool, ishw bool, createpending bool, disabled bool, tracepoint bool, condition *string, ignorecount *int, threadid *int) (*GDBResult, error) {
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
	res := gdb.send(c)
	return createResult(res.(*gdb_result))
}

func (gdb *GDB) break_after(number int, count int) {
}

func (gdb *GDB) break_commands(number int, cmds ...string) {
}

func (gdb *GDB) break_condition(number int, cond string) {
}

func (gdb *GDB) break_delete(number ...int) {
}

func (gdb *GDB) break_disable(number ...int) {
}

func (gdb *GDB) break_enable(number ...int) {
}

func (gdb *GDB) break_info(number int) {
}
