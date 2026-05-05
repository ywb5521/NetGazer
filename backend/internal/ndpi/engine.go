package ndpi

/*
#cgo LDFLAGS: -lndpi -lm -lpthread
#include <ndpi/ndpi_api.h>
#include <ndpi/ndpi_main.h>
#include <stdlib.h>
#include <string.h>

// C helper: NDPI_BITMASK_SET_ALL is a macro and can't be called directly from Go.
static void set_all_bitmask(NDPI_PROTOCOL_BITMASK *bm) {
	NDPI_BITMASK_SET_ALL(*bm);
}
*/
import "C"
import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// FlowKey identifies a unidirectional flow by 5-tuple.
type FlowKey struct {
	SrcIP    string
	DstIP    string
	SrcPort  uint16
	DstPort  uint16
	Protocol uint8 // 6=TCP, 17=UDP
}

// Key returns a canonical string key for the flow.
func (k FlowKey) Key() string {
	return fmt.Sprintf("%s:%d-%s:%d-%d", k.SrcIP, k.SrcPort, k.DstIP, k.DstPort, k.Protocol)
}

type flowEntry struct {
	flow     *C.struct_ndpi_flow_struct
	lastSeen time.Time
}

// Engine wraps an nDPI detection module with per-flow state tracking.
type Engine struct {
	mu       sync.Mutex
	mod      *C.struct_ndpi_detection_module_struct
	flows    map[string]*flowEntry
	flowSize C.u_int32_t
}

// GetNumSupported returns the number of protocols the library can detect.
func GetNumSupported() int {
	// Use a temporary module just to read the count (the library API requires a module)
	mod := C.ndpi_init_detection_module(0)
	if mod == nil {
		return 0
	}
	n := int(C.ndpi_get_num_supported_protocols(mod))
	C.ndpi_exit_detection_module(mod)
	return n
}

// NewEngine creates and initializes an nDPI detection engine.
func NewEngine() (*Engine, error) {
	mod := C.ndpi_init_detection_module(0)
	if mod == nil {
		return nil, fmt.Errorf("ndpi_init_detection_module failed")
	}

	var all C.NDPI_PROTOCOL_BITMASK
	C.set_all_bitmask(&all)
	C.ndpi_set_protocol_detection_bitmask2(mod, &all)

	// Load protocols and categories from default paths (NULL = use built-in)
	C.ndpi_load_protocols_file(mod, nil)
	C.ndpi_load_categories_file(mod, nil)
	C.ndpi_load_risk_domain_file(mod, nil)
	C.ndpi_finalize_initialization(mod)

	flowSize := C.ndpi_detection_get_sizeof_ndpi_flow_struct()

	return &Engine{
		mod:      mod,
		flows:    make(map[string]*flowEntry),
		flowSize: flowSize,
	}, nil
}

// ProtocolResult holds the result of protocol detection.
type ProtocolResult struct {
	ProtoName string
	MasterID  uint16
	AppID     uint16
	Category  string
}

// Detect processes a raw IP packet (starting at IP header, no Ethernet).
// The rawPkt slice must include IP header + transport header + payload.
func (e *Engine) Detect(rawPkt []byte, key FlowKey) (ProtocolResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	entry, ok := e.flows[key.Key()]
	if !ok {
		flowPtr := C.ndpi_flow_malloc(C.size_t(e.flowSize))
		if flowPtr == nil {
			return ProtocolResult{}, fmt.Errorf("ndpi_flow_malloc failed")
		}
		C.memset(flowPtr, 0, C.size_t(e.flowSize))
		entry = &flowEntry{flow: (*C.struct_ndpi_flow_struct)(flowPtr)}
		e.flows[key.Key()] = entry
	}
	entry.lastSeen = time.Now()

	r := C.ndpi_detection_process_packet(
		e.mod,
		entry.flow,
		(*C.uchar)(unsafe.Pointer(&rawPkt[0])),
		C.ushort(len(rawPkt)),
		C.u_int64_t(time.Now().UnixMilli()),
	)

	var result ProtocolResult
	result.MasterID = uint16(r.master_protocol)
	result.AppID = uint16(r.app_protocol)
	result.Category = C.GoString(C.ndpi_category_get_name(e.mod, r.category))

	// Get protocol name
	var nameBuf [64]C.char
	C.ndpi_protocol2name(e.mod, r, &nameBuf[0], 64)
	result.ProtoName = C.GoString(&nameBuf[0])

	return result, nil
}

// IdleCleanup removes flows that haven't been seen within the given duration.
func (e *Engine) IdleCleanup(idleTimeout time.Duration) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := time.Now().Add(-idleTimeout)
	removed := 0
	for key, entry := range e.flows {
		if entry.lastSeen.Before(cutoff) {
			C.ndpi_flow_free(unsafe.Pointer(entry.flow))
			delete(e.flows, key)
			removed++
		}
	}
	return removed
}

// FlowCount returns the number of active tracked flows.
func (e *Engine) FlowCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.flows)
}

// Close releases all resources held by the engine.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, entry := range e.flows {
		C.ndpi_flow_free(unsafe.Pointer(entry.flow))
	}
	e.flows = nil

	if e.mod != nil {
		C.ndpi_exit_detection_module(e.mod)
		e.mod = nil
	}
}
