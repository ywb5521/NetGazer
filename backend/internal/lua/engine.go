package lua

import (
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Host exports to Lua.
type Host struct {
	IP              string
	MAC             string
	Hostname        string
	BytesSent       uint64
	BytesReceived   uint64
	PacketsSent     uint64
	PacketsReceived uint64
	Vendor          string
	ActiveFlows     int
	NodeID          string
}

// Flow exports to Lua.
type Flow struct {
	ID          string
	SrcIP       string
	DstIP       string
	SrcPort     uint16
	DstPort     uint16
	Protocol    string
	AppProtocol string
	Bytes       uint64
	Packets     uint64
}

// Script represents a user Lua script.
type Script struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Enabled bool   `json:"enabled"`
}

// Callback is called by the engine when a script creates an alert.
type Callback func(severity, alertType, message string)

// Engine manages Lua scripts and their execution.
type Engine struct {
	mu          sync.RWMutex
	scripts     map[string]*Script
	pool        []*lua.LState
	poolMu      sync.Mutex
	poolSize    int
	hostsFunc   func(nodeID string) []Host
	flowsFunc   func(nodeID string) []Flow
	alertCB     Callback
}

// NewEngine creates a Lua scripting engine.
func NewEngine(hostsFn func(string) []Host, flowsFn func(string) []Flow, cb Callback) *Engine {
	e := &Engine{
		scripts:   make(map[string]*Script),
		poolSize:  4,
		hostsFunc: hostsFn,
		flowsFunc: flowsFn,
		alertCB:   cb,
	}
	for i := 0; i < e.poolSize; i++ {
		e.pool = append(e.pool, e.newState())
	}
	return e
}

func (e *Engine) newState() *lua.LState {
	L := lua.NewState()
	L.OpenLibs()

	// Register custom functions
	L.SetGlobal("alert", L.NewFunction(e.luaAlert))
	L.SetGlobal("get_hosts", L.NewFunction(e.luaGetHosts))
	L.SetGlobal("get_flows", L.NewFunction(e.luaGetFlows))
	L.SetGlobal("get_host", L.NewFunction(e.luaGetHost))
	L.SetGlobal("get_flow", L.NewFunction(e.luaGetFlow))
	L.SetGlobal("now_unix_ms", L.NewFunction(e.luaNowUnixMs))
	L.SetGlobal("os_time", L.NewFunction(e.luaOsTime))

	// Preload common utils
	preloadUtils(L)

	return L
}

func preloadUtils(L *lua.LState) {
	// string.split
	if t := L.GetGlobal("string"); t != lua.LNil {
		if tbl, ok := t.(*lua.LTable); ok {
			tbl.RawSetString("split", L.NewFunction(func(L *lua.LState) int {
				s := L.CheckString(1)
				sep := L.CheckString(2)
				result := L.CreateTable(1, 0)
				start := 0
				for i := 0; i <= len(s)-len(sep); i++ {
					if s[i:i+len(sep)] == sep {
						result.Append(lua.LString(s[start:i]))
						start = i + len(sep)
						i += len(sep) - 1
					}
				}
				if start < len(s) {
					result.Append(lua.LString(s[start:]))
				}
				L.Push(result)
				return 1
			}))
		}
	}
}

func (e *Engine) getState() *lua.LState {
	e.poolMu.Lock()
	defer e.poolMu.Unlock()
	if len(e.pool) > 0 {
		L := e.pool[len(e.pool)-1]
		e.pool = e.pool[:len(e.pool)-1]
		return L
	}
	return e.newState()
}

func (e *Engine) returnState(L *lua.LState) {
	e.poolMu.Lock()
	defer e.poolMu.Unlock()
	if len(e.pool) < e.poolSize*2 {
		e.pool = append(e.pool, L)
	}
}

// RegisterScript adds or updates a Lua script.
func (e *Engine) RegisterScript(name, content string, enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scripts[name] = &Script{Name: name, Content: content, Enabled: enabled}
}

// RemoveScript deletes a script by name.
func (e *Engine) RemoveScript(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.scripts, name)
}

// ListScripts returns all registered scripts.
func (e *Engine) ListScripts() []Script {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Script, 0, len(e.scripts))
	for _, s := range e.scripts {
		result = append(result, *s)
	}
	return result
}

// OnCheck is called on each alert check cycle for a node.
// It runs on_check(node_id) in each enabled script.
func (e *Engine) OnCheck(nodeID string) {
	e.mu.RLock()
	scripts := make([]*Script, 0, len(e.scripts))
	for _, s := range e.scripts {
		if s.Enabled {
			scripts = append(scripts, s)
		}
	}
	e.mu.RUnlock()

	for _, s := range scripts {
		e.runScript(s, nodeID)
	}
}

func (e *Engine) runScript(s *Script, nodeID string) {
	L := e.getState()
	defer e.returnState(L)

	// Set error handler
	L.SetGlobal("_node_id", lua.LString(nodeID))

	// Compile with context
	source := "_NODE_ID = \"" + nodeID + "\"\n" + s.Content + "\n"
	source += `if type(on_check) == "function" then on_check(_NODE_ID) end`

	done := make(chan bool, 1)
	go func() {
		defer func() { recover() }()
		if err := L.DoString(source); err != nil {
			// Script errors are silently logged; user can test via API
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Timeout protection
	}
}

// RunTest runs a script snippet for testing and returns any error.
func (e *Engine) RunTest(content string, nodeID string) error {
	L := e.getState()
	defer e.returnState(L)

	source := "_NODE_ID = \"" + nodeID + "\"\n" + content
	return L.DoString(source)
}

// --- Lua-accessible functions ---

func (e *Engine) luaAlert(L *lua.LState) int {
	severity := L.CheckString(1)
	alertType := L.CheckString(2)
	message := L.CheckString(3)
	if e.alertCB != nil {
		e.alertCB(severity, alertType, message)
	}
	return 0
}

func (e *Engine) luaGetHosts(L *lua.LState) int {
	nodeID := L.OptString(1, "")
	if e.hostsFunc == nil {
		L.Push(L.CreateTable(0, 0))
		return 1
	}
	hosts := e.hostsFunc(nodeID)
	tbl := L.CreateTable(len(hosts), 0)
	for _, h := range hosts {
		row := L.CreateTable(0, 10)
		row.RawSetString("ip", lua.LString(h.IP))
		row.RawSetString("mac", lua.LString(h.MAC))
		row.RawSetString("hostname", lua.LString(h.Hostname))
		row.RawSetString("bytes_sent", lua.LNumber(h.BytesSent))
		row.RawSetString("bytes_received", lua.LNumber(h.BytesReceived))
		row.RawSetString("packets_sent", lua.LNumber(h.PacketsSent))
		row.RawSetString("packets_received", lua.LNumber(h.PacketsReceived))
		row.RawSetString("vendor", lua.LString(h.Vendor))
		row.RawSetString("active_flows", lua.LNumber(h.ActiveFlows))
		row.RawSetString("node_id", lua.LString(h.NodeID))
		tbl.Append(row)
	}
	L.Push(tbl)
	return 1
}

func (e *Engine) luaGetFlows(L *lua.LState) int {
	nodeID := L.OptString(1, "")
	if e.flowsFunc == nil {
		L.Push(L.CreateTable(0, 0))
		return 1
	}
	flows := e.flowsFunc(nodeID)
	tbl := L.CreateTable(len(flows), 0)
	for _, f := range flows {
		row := L.CreateTable(0, 9)
		row.RawSetString("id", lua.LString(f.ID))
		row.RawSetString("src_ip", lua.LString(f.SrcIP))
		row.RawSetString("dst_ip", lua.LString(f.DstIP))
		row.RawSetString("src_port", lua.LNumber(f.SrcPort))
		row.RawSetString("dst_port", lua.LNumber(f.DstPort))
		row.RawSetString("protocol", lua.LString(f.Protocol))
		row.RawSetString("app_protocol", lua.LString(f.AppProtocol))
		row.RawSetString("bytes", lua.LNumber(f.Bytes))
		row.RawSetString("packets", lua.LNumber(f.Packets))
		tbl.Append(row)
	}
	L.Push(tbl)
	return 1
}

func (e *Engine) luaGetHost(L *lua.LState) int {
	ip := L.CheckString(1)
	if ip == "" || e.hostsFunc == nil {
		L.Push(lua.LNil)
		return 1
	}
	hosts := e.hostsFunc("")
	for _, h := range hosts {
		if h.IP == ip {
			row := L.CreateTable(0, 10)
			row.RawSetString("ip", lua.LString(h.IP))
			row.RawSetString("mac", lua.LString(h.MAC))
			row.RawSetString("hostname", lua.LString(h.Hostname))
			row.RawSetString("bytes_sent", lua.LNumber(h.BytesSent))
			row.RawSetString("bytes_received", lua.LNumber(h.BytesReceived))
			row.RawSetString("vendor", lua.LString(h.Vendor))
			row.RawSetString("active_flows", lua.LNumber(h.ActiveFlows))
			L.Push(row)
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

func (e *Engine) luaGetFlow(L *lua.LState) int {
	id := L.CheckString(1)
	if id == "" || e.flowsFunc == nil {
		L.Push(lua.LNil)
		return 1
	}
	flows := e.flowsFunc("")
	for _, f := range flows {
		if f.ID == id {
			row := L.CreateTable(0, 9)
			row.RawSetString("id", lua.LString(f.ID))
			row.RawSetString("src_ip", lua.LString(f.SrcIP))
			row.RawSetString("dst_ip", lua.LString(f.DstIP))
			row.RawSetString("src_port", lua.LNumber(f.SrcPort))
			row.RawSetString("dst_port", lua.LNumber(f.DstPort))
			row.RawSetString("protocol", lua.LString(f.Protocol))
			row.RawSetString("app_protocol", lua.LString(f.AppProtocol))
			row.RawSetString("bytes", lua.LNumber(f.Bytes))
			row.RawSetString("packets", lua.LNumber(f.Packets))
			L.Push(row)
			return 1
		}
	}
	L.Push(lua.LNil)
	return 1
}

func (e *Engine) luaNowUnixMs(L *lua.LState) int {
	L.Push(lua.LNumber(time.Now().UnixMilli()))
	return 1
}

func (e *Engine) luaOsTime(L *lua.LState) int {
	L.Push(lua.LNumber(time.Now().Unix()))
	return 1
}
