package gdbmi

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
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

func equals(s1 string, s2 string) bool {
	return bytes.Equal([]byte(s1), []byte(s2))
}

func mapValueAsString(m map[string]interface{}, key string, def string) string {
	v, ok := m[key]
	if ok {
		return v.(string)
	}
	return def
}

func cutoff(line string, prefix string, removeQuotes bool) string {
	start := len(prefix)
	end := len(line)
	if removeQuotes {
		start += 1
		end -= 1
	}
	return string([]byte(line)[start:end])
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
	Fill(fields map[string]string) error
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
func (r *gdb_response_type) Fill(fields map[string]string) error {
	token, ok := fields["token"]
	if ok && len(token) > 0 {
		tok, perr := strconv.ParseInt(token, 10, 64)
		if perr != nil {
			return fmt.Errorf("cannot parse token %s", token)
		} else {
			r.token = tok
		}
	}
	r.line = fields["message"]
	return nil
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

func (c *gdb_command) add_option_stringvalue(opt string, optparam *string) *gdb_command {
	if optparam != nil {
		c.options = append(c.options, fmt.Sprintf("-%s %s", opt, *optparam))
	}
	return c
}
func (c *gdb_command) add_option_intvalue(opt string, optparam *int) *gdb_command {
	if optparam != nil {
		c.options = append(c.options, fmt.Sprintf("-%s %d", opt, *optparam))
	}
	return c
}

func (c *gdb_command) add_option(opt string) *gdb_command {
	c.options = append(c.options, fmt.Sprintf("-%s", opt))
	return c
}
func (c *gdb_command) add_option_when(flg bool, opt string) *gdb_command {
	if flg {
		return c.add_option(opt)
	}
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
	Type         GDBResultType `json:"type"`
	Results      string        `json:"results"`
	ErrorMessage string        `json:"errorMessage"`
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

type stopReasons struct {
	stopReason2Id map[string]GDBStopReason
	stopId2Reason map[GDBStopReason]string
}

func (st *stopReasons) add(name string, r GDBStopReason) {
	st.stopId2Reason[r] = name
	st.stopReason2Id[name] = r
}
func newStopReasons() *stopReasons {
	var sr stopReasons
	sr.stopReason2Id = make(map[string]GDBStopReason)
	sr.stopId2Reason = make(map[GDBStopReason]string)
	return &sr
}

func StopReasonWithName(n string) (GDBStopReason, bool) {
	sr, ok := allStopReasons.stopReason2Id[n]
	return sr, ok
}

type asyncTypes struct {
	asyncName2TypeId map[string]GDBAsyncType
	asyncTypeId2Name map[GDBAsyncType]string
}

func (st *asyncTypes) add(name string, r GDBAsyncType) {
	st.asyncTypeId2Name[r] = name
	st.asyncName2TypeId[name] = r
}
func newAsyncTypes() *asyncTypes {
	var at asyncTypes
	at.asyncName2TypeId = make(map[string]GDBAsyncType)
	at.asyncTypeId2Name = make(map[GDBAsyncType]string)
	return &at
}

func AsyncTypeWithName(n string) (GDBAsyncType, bool) {
	at, ok := allAsyncTypes.asyncName2TypeId[n]
	return at, ok
}

type resultTypes struct {
	resultId2Type map[GDBResultType]string
	resultType2Id map[string]GDBResultType
}

func (rt *resultTypes) add(name string, r GDBResultType) {
	rt.resultId2Type[r] = name
	rt.resultType2Id[name] = r
}
func newResultTypes() *resultTypes {
	var rt resultTypes
	rt.resultType2Id = make(map[string]GDBResultType)
	rt.resultId2Type = make(map[GDBResultType]string)
	return &rt
}

func ResultTypeWithName(n string) (GDBResultType, bool) {
	rt, ok := allResultTypes.resultType2Id[n]
	return rt, ok
}

type breakpointTypes struct {
	breakId2Name map[BreakpointType]string
	breakName2Id map[string]BreakpointType
}

func newBreakpointTypes() *breakpointTypes {
	var bp breakpointTypes
	bp.breakId2Name = make(map[BreakpointType]string)
	bp.breakName2Id = make(map[string]BreakpointType)
	return &bp
}
func (bp *breakpointTypes) add(name string, r BreakpointType) {
	bp.breakId2Name[r] = name
	bp.breakName2Id[name] = r
}
func BreakpointWithName(n string) (BreakpointType, bool) {
	bp, ok := allBreakpointTypes.breakName2Id[n]
	return bp, ok
}

type breakpointDispositionTypes struct {
	breakId2Name map[BreakpointDispositionType]string
	breakName2Id map[string]BreakpointDispositionType
}

func newBreakpointDispositionTypes() *breakpointDispositionTypes {
	var bp breakpointDispositionTypes
	bp.breakId2Name = make(map[BreakpointDispositionType]string)
	bp.breakName2Id = make(map[string]BreakpointDispositionType)
	return &bp
}
func (bp *breakpointDispositionTypes) add(name string, r BreakpointDispositionType) {
	bp.breakId2Name[r] = name
	bp.breakName2Id[name] = r
}
func BreakpointDispositionWithName(n string) (BreakpointDispositionType, bool) {
	bp, ok := allBreakpointDispositionTypes.breakName2Id[n]
	return bp, ok
}

var (
	allStopReasons                = newStopReasons()
	allAsyncTypes                 = newAsyncTypes()
	allResultTypes                = newResultTypes()
	allBreakpointTypes            = newBreakpointTypes()
	allBreakpointDispositionTypes = newBreakpointDispositionTypes()
)

func init() {

	allBreakpointTypes.add("breakpoint", BP_breakpoint)
	allBreakpointTypes.add("catchpoint", BP_catchpoint)

	allBreakpointDispositionTypes.add("del", BP_breakpointDisposition_delete)
	allBreakpointDispositionTypes.add("keep", BP_breakpointDisposition_keep)

	allResultTypes.add("done", Result_done)
	allResultTypes.add("running", Result_running)
	allResultTypes.add("connected", Result_connected)
	allResultTypes.add("error", Result_error)
	allResultTypes.add("exit", Result_exit)

	allAsyncTypes.add("running", Async_running)
	allAsyncTypes.add("stopped", Async_stopped)
	allAsyncTypes.add("thread-group-added", Async_thread_group_added)
	allAsyncTypes.add("thread-group-removed", Async_thread_group_removed)
	allAsyncTypes.add("thread-group-started", Async_thread_group_started)
	allAsyncTypes.add("thread-group-exited", Async_thread_group_exited)
	allAsyncTypes.add("thread-created", Async_thread_created)
	allAsyncTypes.add("thread-exited", Async_thread_exited)
	allAsyncTypes.add("thread-selected", Async_thread_selected)
	allAsyncTypes.add("library-loaded", Async_library_loaded)
	allAsyncTypes.add("library-unloaded", Async_library_unloaded)
	allAsyncTypes.add("traceframe-changed", Async_traceframe_changed)
	allAsyncTypes.add("tsv-created", Async_tsv_created)
	allAsyncTypes.add("tsv-deleted", Async_tsv_deleted)
	allAsyncTypes.add("tsv-modified", Async_tsv_modified)
	allAsyncTypes.add("breakpoint-created", Async_breakpoint_created)
	allAsyncTypes.add("breakpoint-modified", Async_breakpoint_modified)
	allAsyncTypes.add("breakpoint-deleted", Async_breakpoint_deleted)
	allAsyncTypes.add("record-started", Async_record_started)
	allAsyncTypes.add("record-stopped", Async_record_stopped)
	allAsyncTypes.add("cmd-param-changed", Async_cmd_param_changed)
	allAsyncTypes.add("memory-changed", Async_memory_changed)

	allStopReasons.add("breakpoint-hit", Async_stopped_breakpoint_hit)
	allStopReasons.add("breakpoint-hit", Async_stopped_breakpoint_hit)
	allStopReasons.add("watchpoint-trigger", Async_stopped_watchpoint_trigger)
	allStopReasons.add("read-watchpoint-trigger", Async_stopped_read_watchpoint_trigger)
	allStopReasons.add("access-watchpoint-trigger", Async_stopped_access_watchpoint_trigger)
	allStopReasons.add("function-finished", Async_stopped_function_finished)
	allStopReasons.add("location-reached", Async_stopped_location_reached)
	allStopReasons.add("watchpoint-scope", Async_stopped_watchpoint_scope)
	allStopReasons.add("end-stepping-range", Async_stopped_end_stepping_range)
	allStopReasons.add("exited-signalled", Async_stopped_exited_signalled)
	allStopReasons.add("exited", Async_stopped_exited)
	allStopReasons.add("exited-normally", Async_stopped_exited_normally)
	allStopReasons.add("signal-received", Async_stopped_signal_received)
	allStopReasons.add("solib-event", Async_stopped_solib_event)
	allStopReasons.add("fork", Async_stopped_fork)
	allStopReasons.add("vfork", Async_stopped_vfork)
	allStopReasons.add("syscall-entry", Async_stopped_syscall_entry)
	allStopReasons.add("exec", Async_stopped_exec)
}

func (rt GDBResultType) String() string {
	return allResultTypes.resultId2Type[rt]
}

func (sr GDBStopReason) String() string {
	return allStopReasons.stopId2Reason[sr]
}

func (at GDBAsyncType) String() string {
	return allAsyncTypes.asyncTypeId2Name[at]
}

func (bp BreakpointType) String() string {
	return allBreakpointTypes.breakId2Name[bp]
}

func (bp BreakpointDispositionType) String() string {
	return allBreakpointDispositionTypes.breakId2Name[bp]
}

// This event happens async in GDB. Not all fields are filled, but the Type is never empty. Depending on
// the Type the other fields are filled or not. Look at the GDB/MI documentation to find more information
// about the fields.
type GDBEvent struct {
	Type             GDBAsyncType  `json:"type"`
	StopReason       GDBStopReason `json:"stopReason"`
	ThreadId         string        `json:"threadId"`
	ThreadGroupid    string        `json:"threadGroupId"`
	StoppedThreads   string        `json:"stoppendThreads"`
	StopCore         string        `json:"stopCopre"`
	Pid              int           `json:"pid"`
	ExitCode         int           `json:"exitCode"`
	TraceFrameNumber int           `json:"traceFrameNumber"`
	TracePointNumber int           `json:"tracePointNumber"`
	TsvName          string        `json:"tsvName"`
	TsvValue         string        `json:"tsvValue"`
	TsvInitial       string        `json:"tsvInitial"`
	CmdParam         string        `json:"cmdParam"`
	CmdValue         string        `json:"cmdValue"`
	MemoryAddress    int           `json:"memoryAddress"`
	MemoryLen        int           `json:"memoryLen"`
	MemoryTypeCode   bool          `json:"memoryTypeCode"`
	BreakpointNumber string        `json:"breakpointNumber"`
}

type GDBTargetConsoleEvent struct {
	Line string `json:"line"`
}

// A running debugger
type GDB struct {
	Event           chan GDBEvent
	Target          chan GDBTargetConsoleEvent
	DebuggerProcess *os.Process

	quit     chan bool
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	stdin    io.WriteCloser
	commands chan gdb_command
	result   chan gdb_response
	send     func(cmd *gdb_command) (*GDBResult, error)
	start    func(gdb *GDB, gdbpath string, gdbparms []string, env []string) error
	gdbpath  string
}

func NewGDB(gdbpath string) *GDB {
	gdb := new(GDB)
	gdb.Event = make(chan GDBEvent)
	gdb.Target = make(chan GDBTargetConsoleEvent)

	gdb.quit = make(chan bool)
	gdb.commands = make(chan gdb_command)
	gdb.result = make(chan gdb_response)
	gdb.send = gdb.gdbsend
	gdb.start = startupGDB
	gdb.gdbpath = gdbpath

	return gdb
}
func (gdb *GDB) Start(executable string, env ...string) error {
	gdbargs := []string{"-q", "-i", "mi"}
	gdbargs = append(gdbargs, executable)

	if err := gdb.start(gdb, gdb.gdbpath, gdbargs, env); err != nil {
		return err
	}
	return nil
}

func (gdb *GDB) Close() {
	close(gdb.quit)

	/*
		gdb.stdin.Close()
		gdb.stdout.Close()
		gdb.stderr.Close()
		close(gdb.Event)
		close(gdb.Target) */
}

func startupGDB(gdb *GDB, gdbpath string, gdbargs []string, env []string) error {
	cmd := exec.Command(gdbpath, gdbargs...)
	cmd.Env = env
	pipe, err := cmd.StdoutPipe()

	if err != nil {
		return err
	}
	gdb.stdout = pipe
	go gdb.parse_gdb_output()

	pipe, err = cmd.StderrPipe()
	if err != nil {
		return err
	}
	gdb.stderr = pipe
	ipipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	gdb.stdin = ipipe
	if err := cmd.Start(); err != nil {
		return err
	}
	gdb.DebuggerProcess = cmd.Process
	go func() {
		open_commands := make(map[int64]*gdb_command)
		for {
			select {
			case <-gdb.quit:
				close(gdb.commands)
				close(gdb.Target)
				close(gdb.Event)
				return
			case c, ok := <-gdb.commands:
				if !ok {
					return
				}
				gdb.send_to_gdb(&c)
				open_commands[c.token] = &c
			case r, ok := <-gdb.result:
				if !ok {
					return
				}
				switch rt := r.(type) {
				case *gdb_result:
					waiting_cmd, ok := open_commands[r.Token()]
					if ok {
						waiting_cmd.result <- r
					}
				case *gdb_console_output:
				case *gdb_target_output:
					ev := new(GDBTargetConsoleEvent)
					ev.Line = r.Line()
					go func() {
						gdb.Target <- *ev
					}()
				case *gdb_log_output:
					fmt.Printf(" LOG ---> %s\n", r.Line())
					//log.Printf("LOG: %+v", r)
				case *gdb_async:
					ev, err := createAsync(rt)
					if err != nil {
						//log.Printf("Async Event Error: %s", err)
					} else {
						go func() {
							gdb.Event <- *ev
						}()
					}
				}
			}
		}
	}()
	return nil
}

func (gdb *GDB) parse_gdb_output() {
	buf := bufio.NewReader(gdb.stdout)
	for {
		var ln []byte
		ln, err := buf.ReadBytes('\n')
		if err != nil {
			close(gdb.result)
			return
		}
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
				rsp := new(gdb_target_output)
				rsp.line = sline
				gdb.result <- rsp
			}
		}

	}
}

func (gdb *GDB) send_to_gdb(cmd *gdb_command) {
	fmt.Fprintln(gdb.stdin, cmd.dump_mi())
}

func (gdb *GDB) gdbsend(cmd *gdb_command) (*GDBResult, error) {
	gdb.commands <- *cmd
	rsp := <-cmd.result
	result, err := createResult(rsp.(*gdb_result))
	if err == nil {
		if result.Type == Result_error {
			return nil, fmt.Errorf("%s", result.ErrorMessage)
		}
		return result, nil
	}
	return nil, err
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
		result.StopCore, _ = params["core"]
		reason, _ := params["reason"]
		sr, ok := StopReasonWithName(reason)
		if !ok {
			return nil, fmt.Errorf("Error: unknown stopreaseon: %s", reason)
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
	} else if strings.HasPrefix(res.Line(), Result_running.String()) {
		result.Type = Result_running
		return &result, nil
	} else if strings.HasPrefix(res.Line(), Result_connected.String()) {
		result.Type = Result_connected
		return &result, nil
	} else if strings.HasPrefix(res.Line(), Result_error.String()) {
		parts := strings.SplitN(res.Line(), ",", 2)
		if len(parts) > 1 {
			result.ErrorMessage = parts[1]
		}
		result.Type = Result_error
		return &result, nil
	} else if strings.HasPrefix(res.Line(), Result_exit.String()) {
		result.Type = Result_exit
		return &result, nil
	}
	return nil, fmt.Errorf("unknown result indication '%s'", res.Line())
}

func (gdb *GDB) Exec_arguments(args ...string) (*GDBResult, error) {
	c := newCommand("exec-arguments")
	for _, a := range args {
		c.add_param(a)
	}
	return gdb.send(c)
}

func (gdb *GDB) Environment_cd(dir string) (*GDBResult, error) {
	c := newCommand("environment-cd").add_param(dir)
	return gdb.send(c)
}

func (gdb *GDB) Environment_directory(reset bool, dirs ...string) (string, error) {
	return gdb.environment_path_query("environment-directory", "source-path=", reset, dirs...)
}
func (gdb *GDB) Environment_path(reset bool, dirs ...string) (string, error) {
	return gdb.environment_path_query("environment-path", "path=", reset, dirs...)
}
func (gdb *GDB) Environment_pwd() (string, error) {
	return gdb.environment_path_query("environment-pwd", "cwd=", false, []string{}...)
}
func (gdb *GDB) environment_path_query(gfunc string, prefix string, reset bool, dirs ...string) (string, error) {
	c := newCommand(gfunc).add_option_when(reset, "-r")
	for _, d := range dirs {
		c.add_param(fmt.Sprintf("\"%s\"", d))
	}
	res, err := gdb.send(c)
	if err != nil {
		return "", err
	}
	sourcep := cutoff(res.Results, prefix, true)
	return sourcep, nil
}

func (gdb *GDB) Exec_next() {
	c := newCommand("exec-next")
	gdb.send(c)
}

func (gdb *GDB) Exec_nexti(reverse bool) {
	c := newCommand("exec-next-instruction")
	if reverse {
		c.add_option("--reverse")
	}
	gdb.send(c)
}

func (gdb *GDB) Exec_run(all bool, threadgroup *int) (*GDBResult, error) {
	c := newCommand("exec-run")
	if all {
		c.add_option("--all")
	}
	if threadgroup != nil {
		c.add_option_intvalue("--thread-group", threadgroup)
	}
	return gdb.send(c)
}

func (gdb *GDB) Gdb_exit() {
	gdb.send(newCommand("gdb-exit"))
}
