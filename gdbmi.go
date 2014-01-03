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

const (
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

var (
	stopReason2Id    = make(map[string]GDBStopReason)
	stopId2Reason    = make(map[GDBStopReason]string)
	asyncName2TypeId = make(map[string]GDBAsyncType)
	asyncTypeId2Name = make(map[GDBAsyncType]string)
	resultId2Type    = make(map[GDBResultType]string)
	resultType2Id    = make(map[string]GDBResultType)
)

func init() {
	resultType2Id["done"] = Result_done
	resultType2Id["running"] = Result_running
	resultType2Id["connected"] = Result_connected
	resultType2Id["error"] = Result_error
	resultType2Id["exit"] = Result_exit
	for k, v := range resultType2Id {
		resultId2Type[v] = k
	}
	asyncName2TypeId["running"] = Async_running
	asyncName2TypeId["stopped"] = Async_stopped
	asyncName2TypeId["thread-group-added"] = Async_thread_group_added
	asyncName2TypeId["thread-group-removed"] = Async_thread_group_removed
	asyncName2TypeId["thread-group-started"] = Async_thread_group_started
	asyncName2TypeId["thread-group-exited"] = Async_thread_group_exited
	asyncName2TypeId["thread-created"] = Async_thread_created
	asyncName2TypeId["thread-exited"] = Async_thread_exited
	asyncName2TypeId["thread-selected"] = Async_thread_selected
	asyncName2TypeId["library-loaded"] = Async_library_loaded
	asyncName2TypeId["library-unloaded"] = Async_library_unloaded
	asyncName2TypeId["traceframe-changed"] = Async_traceframe_changed
	asyncName2TypeId["tsv-created"] = Async_tsv_created
	asyncName2TypeId["tsv-deleted"] = Async_tsv_deleted
	asyncName2TypeId["tsv-modified"] = Async_tsv_modified
	asyncName2TypeId["breakpoint-created"] = Async_breakpoint_created
	asyncName2TypeId["breakpoint-modified"] = Async_breakpoint_modified
	asyncName2TypeId["breakpoint-deleted"] = Async_breakpoint_deleted
	asyncName2TypeId["record-started"] = Async_record_started
	asyncName2TypeId["record-stopped"] = Async_record_stopped
	asyncName2TypeId["cmd-param-changed"] = Async_cmd_param_changed
	asyncName2TypeId["memory-changed"] = Async_memory_changed
	for k, v := range asyncName2TypeId {
		asyncTypeId2Name[v] = k
	}

	stopReason2Id["breakpoint-hit"] = Async_stopped_breakpoint_hit
	stopReason2Id["watchpoint-trigger"] = Async_stopped_watchpoint_trigger
	stopReason2Id["read-watchpoint-trigger"] = Async_stopped_read_watchpoint_trigger
	stopReason2Id["access-watchpoint-trigger"] = Async_stopped_access_watchpoint_trigger
	stopReason2Id["function-finished"] = Async_stopped_function_finished
	stopReason2Id["location-reached"] = Async_stopped_location_reached
	stopReason2Id["watchpoint-scope"] = Async_stopped_watchpoint_scope
	stopReason2Id["end-stepping-range"] = Async_stopped_end_stepping_range
	stopReason2Id["exited-signalled"] = Async_stopped_exited_signalled
	stopReason2Id["exited"] = Async_stopped_exited
	stopReason2Id["exited-normally"] = Async_stopped_exited_normally
	stopReason2Id["signal-received"] = Async_stopped_signal_received
	stopReason2Id["solib-event"] = Async_stopped_solib_event
	stopReason2Id["fork"] = Async_stopped_fork
	stopReason2Id["vfork"] = Async_stopped_vfork
	stopReason2Id["syscall-entry"] = Async_stopped_syscall_entry
	stopReason2Id["exec"] = Async_stopped_exec
	for k, v := range stopReason2Id {
		stopId2Reason[v] = k
	}
}

func (rt GDBResultType) String() string {
	return resultId2Type[rt]
}

func (sr GDBStopReason) String() string {
	return stopId2Reason[sr]
}

func (at GDBAsyncType) String() string {
	return asyncTypeId2Name[at]
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
	MemoryAddress    int64
	MemoryLen        int64
	MemoryTypeCode   bool
}

// A running debugger
type GDB struct {
	Event    chan GDBEvent
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	stdin    io.WriteCloser
	commands chan gdb_command
	result   chan gdb_response
	console  chan gdb_response
	async    chan gdb_response
	target   chan gdb_response
	log      chan gdb_response
}

func NewGDB(gdbpath string, executable string, params []string, env []string) (*GDB, error) {
	gdb := new(GDB)
	gdb.Event = make(chan GDBEvent)
	gdb.commands = make(chan gdb_command)
	gdb.result = make(chan gdb_response)
	gdb.console = make(chan gdb_response)
	gdb.async = make(chan gdb_response)
	gdb.target = make(chan gdb_response)
	gdb.log = make(chan gdb_response)
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
				waiting_cmd, ok := open_commands[r.Token()]
				if ok {
					waiting_cmd.result <- r
				}
			case r := <-gdb.console:
				log.Printf("CONSOLE: %+v", r)
			case r := <-gdb.log:
				log.Printf("LOG: %+v", r)
			case r := <-gdb.target:
				log.Printf("TARGET: %+v", r)
			case r := <-gdb.async:
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
					switch rsp.(type) {
					case *gdb_result:
						gdb.result <- rsp
					case *gdb_async:
						gdb.async <- rsp
					case *gdb_console_output:
						gdb.console <- rsp
					case *gdb_log_output:
						gdb.log <- rsp
					case *gdb_target_output:
						gdb.target <- rsp
					}
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
	t, ok := asyncName2TypeId[tp]
	if !ok {
		return Async_unknown
	}
	return t
}

func createAsync(res *gdb_async) (*GDBEvent, error) {
	var result GDBEvent
	toks := strings.SplitN(res.Line(), ",", 2)
	switch asyncTypeFromString(toks[0]) {
	case Async_running:
		result.ThreadId = async_running_line.ReplaceAllString(res.Line(), "$1")
		result.Type = Async_running
		return &result, nil
	case Async_stopped:
		sub := []byte(res.Line())[len(asyncTypeId2Name[Async_stopped])+1:]
		params := splitKVList(string(sub))
		result.ThreadId, _ = params["thread-id"]
		result.StoppedThreads, _ = params["stopped-threads"]
		result.StopeCore, _ = params["core"]
		reason, _ := params["reason"]
		sr, ok := stopReason2Id[reason]
		if !ok {
			log.Printf("Error: unknown stopreaseon: %s\n", reason)
		} else {
			result.StopReason = sr
		}
		result.Type = Async_stopped
		return &result, nil
	case Async_thread_group_started:
		sub := []byte(res.Line())[len(asyncTypeId2Name[Async_thread_group_started])+1:]
		params := splitKVList(string(sub))
		result.Type = Async_thread_group_started
		result.ThreadGroupid, _ = params["id"]
		_, err := fmt.Sscanf(params["pid"], "%d", &result.Pid)
		return &result, err
	case Async_thread_group_exited:
		sub := []byte(res.Line())[len(asyncTypeId2Name[Async_thread_group_exited])+1:]
		params := splitKVList(string(sub))
		result.Type = Async_thread_group_exited
		result.ThreadGroupid, _ = params["id"]
		_, err := fmt.Sscanf(params["exit-code"], "%d", &result.ExitCode)
		return &result, err
	case Async_thread_created:
		sub := []byte(res.Line())[len(asyncTypeId2Name[Async_thread_created])+1:]
		params := splitKVList(string(sub))
		result.Type = Async_thread_created
		result.ThreadId, _ = params["id"]
		result.ThreadGroupid, _ = params["gid"]
		return &result, nil
	case Async_thread_exited:
		sub := []byte(res.Line())[len(asyncTypeId2Name[Async_thread_exited])+1:]
		params := splitKVList(string(sub))
		result.Type = Async_thread_exited
		result.ThreadId, _ = params["id"]
		result.ThreadGroupid, _ = params["gid"]
		return &result, nil
	default:
		return nil, fmt.Errorf("unknown async message: %s", res.Line())
	}
}

func createResult(res *gdb_result) (*GDBResult, error) {
	var result GDBResult
	if strings.HasPrefix(res.Line(), resultId2Type[Result_done]) {
		parts := strings.SplitN(res.Line(), ",", 2)
		if len(parts) > 1 {
			result.Results = parts[1]
		}
		result.Type = Result_done
		return &result, nil
	} else if strings.HasPrefix(res.Line(), resultId2Type[Result_running]) {
		result.Type = Result_running
		return &result, nil
	} else if strings.HasPrefix(res.Line(), resultId2Type[Result_connected]) {
		result.Type = Result_connected
		return &result, nil
	} else if strings.HasPrefix(res.Line(), resultId2Type[Result_error]) {
		parts := strings.SplitN(res.Line(), ",", 2)
		if len(parts) > 1 {
			result.ErrorMessage = parts[1]
		}
		result.Type = Result_error
		return &result, nil
	} else if strings.HasPrefix(res.Line(), resultId2Type[Result_exit]) {
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
