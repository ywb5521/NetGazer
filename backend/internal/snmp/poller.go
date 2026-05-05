package snmp

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
)

// DeviceConfig defines an SNMP device to poll.
type DeviceConfig struct {
	Name      string // display name (falls back to sysName if empty)
	Target    string // IP or hostname
	Community string // SNMP v1/v2c community string
	Version   string // "1", "2c", or "3"
	Port      uint16 // default 161
	// SNMPv3 USM fields
	V3Username       string // USM security name
	V3AuthProtocol   string // "MD5", "SHA", "SHA256"
	V3AuthPassphrase string
	V3PrivProtocol   string // "DES", "AES"
	V3PrivPassphrase string
	V3SecurityLevel  string // "noAuthNoPriv", "authNoPriv", "authPriv"
	// Custom OID polling
	CustomOIDs []CustomOID
}

// CustomOID defines a custom SNMP OID to poll.
type CustomOID struct {
	OID  string `json:"oid"`
	Name string `json:"name"`
	Type string `json:"type"` // "gauge", "counter", "string", "integer"
}

// InterfaceStat holds per-interface SNMP data.
type InterfaceStat struct {
	Index      int
	Name       string
	Alias      string
	InOctets   uint64
	OutOctets  uint64
	InErrors   uint64
	OutErrors  uint64
	Speed      uint64
	OperStatus int // 1=up, 2=down
	AdminStatus int
}

// DeviceSnapshot holds a complete snapshot of a device.
type DeviceSnapshot struct {
	DeviceName   string
	SysName      string
	SysDescr     string
	SysUptime    uint64
	SysContact   string
	SysLocation  string
	Interfaces   []InterfaceStat
	CustomValues map[string]string // custom OID name → value
	NodeID       string
	Timestamp    time.Time
}

// Callback is called with each device snapshot.
type Callback func(snapshot DeviceSnapshot)

// Poller polls SNMP devices periodically.
type Poller struct {
	devices  []DeviceConfig
	interval time.Duration
	callback Callback
	mu       sync.Mutex
	prevIn   map[string]uint64 // nodeID:ifIndex -> previous InOctets
	prevOut  map[string]uint64 // nodeID:ifIndex -> previous OutOctets
	prevTime map[string]time.Time
}

// NewPoller creates a new SNMP poller.
func NewPoller(devices []DeviceConfig, interval time.Duration, cb Callback) *Poller {
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &Poller{
		devices:  devices,
		interval: interval,
		callback: cb,
		prevIn:   make(map[string]uint64),
		prevOut:  make(map[string]uint64),
		prevTime: make(map[string]time.Time),
	}
}

// Start begins polling all configured devices.
func (p *Poller) Start(ctx context.Context) {
	if len(p.devices) == 0 {
		return
	}
	log.Printf("[snmp] starting poller with %d device(s), interval=%s", len(p.devices), p.interval)

	// Initial poll immediately
	p.pollAll()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollAll()
		}
	}
}

func (p *Poller) pollAll() {
	for _, dev := range p.devices {
		snapshot, err := p.pollDevice(dev)
		if err != nil {
			log.Printf("[snmp] poll %s (%s) failed: %v", dev.Name, dev.Target, err)
			continue
		}
		if p.callback != nil {
			p.callback(snapshot)
		}
	}
}

func (p *Poller) pollDevice(dev DeviceConfig) (DeviceSnapshot, error) {
	port := dev.Port
	if port == 0 {
		port = 161
	}

	gs := &gosnmp.GoSNMP{
		Target:  dev.Target,
		Port:    port,
		Timeout: time.Duration(5) * time.Second,
		Retries: 1,
	}

	if dev.Version == "3" {
		gs.Version = gosnmp.Version3
		gs.SecurityModel = gosnmp.UserSecurityModel
		secParams := &gosnmp.UsmSecurityParameters{
			UserName:                 dev.V3Username,
			AuthenticationProtocol:   authProtocol(dev.V3AuthProtocol),
			AuthenticationPassphrase: dev.V3AuthPassphrase,
			PrivacyProtocol:          privProtocol(dev.V3PrivProtocol),
			PrivacyPassphrase:        dev.V3PrivPassphrase,
		}
		gs.SecurityParameters = secParams
		switch dev.V3SecurityLevel {
		case "authPriv":
			gs.MsgFlags = gosnmp.AuthPriv
		case "authNoPriv":
			gs.MsgFlags = gosnmp.AuthNoPriv
		default:
			gs.MsgFlags = gosnmp.NoAuthNoPriv
		}
	} else {
		gs.Community = dev.Community
		if dev.Version == "1" {
			gs.Version = gosnmp.Version1
		} else {
			gs.Version = gosnmp.Version2c
		}
	}

	if err := gs.Connect(); err != nil {
		return DeviceSnapshot{}, fmt.Errorf("connect: %w", err)
	}
	defer gs.Conn.Close()

	nodeID := "snmp:" + dev.Target

	// Poll system info
	sysName := p.getOne(gs, ".1.3.6.1.2.1.1.5.0")
	sysDescr := p.getOne(gs, ".1.3.6.1.2.1.1.1.0")
	sysUptime := p.getOneUint(gs, ".1.3.6.1.2.1.1.3.0")
	sysContact := p.getOne(gs, ".1.3.6.1.2.1.1.4.0")
	sysLocation := p.getOne(gs, ".1.3.6.1.2.1.1.6.0")

	deviceName := dev.Name
	if deviceName == "" && sysName != "" {
		deviceName = sysName
	}
	if deviceName == "" {
		deviceName = dev.Target
	}

	// Walk interfaces table
	ifaces := p.walkInterfaces(gs, nodeID)

	// Poll custom OIDs
	customValues := make(map[string]string)
	for _, coid := range dev.CustomOIDs {
		val := p.getOne(gs, coid.OID)
		customValues[coid.Name] = val
	}

	snapshot := DeviceSnapshot{
		DeviceName:   deviceName,
		SysName:      sysName,
		SysDescr:     sysDescr,
		SysUptime:    sysUptime,
		SysContact:   sysContact,
		SysLocation:  sysLocation,
		Interfaces:   ifaces,
		CustomValues: customValues,
		NodeID:       nodeID,
		Timestamp:    time.Now(),
	}

	return snapshot, nil
}

func authProtocol(name string) gosnmp.SnmpV3AuthProtocol {
	switch name {
	case "MD5":
		return gosnmp.MD5
	case "SHA256":
		return gosnmp.SHA256
	default:
		return gosnmp.SHA
	}
}

func privProtocol(name string) gosnmp.SnmpV3PrivProtocol {
	switch name {
	case "AES":
		return gosnmp.AES
	default:
		return gosnmp.DES
	}
}

func (p *Poller) getOne(gs *gosnmp.GoSNMP, oid string) string {
	result, err := gs.Get([]string{oid})
	if err != nil || len(result.Variables) == 0 {
		return ""
	}
	v := result.Variables[0]
	switch v.Type {
	case gosnmp.OctetString:
		return string(v.Value.([]byte))
	default:
		return fmt.Sprintf("%v", v.Value)
	}
}

func (p *Poller) getOneUint(gs *gosnmp.GoSNMP, oid string) uint64 {
	result, err := gs.Get([]string{oid})
	if err != nil || len(result.Variables) == 0 {
		return 0
	}
	return gosnmp.ToBigInt(result.Variables[0].Value).Uint64()
}

func (p *Poller) walkInterfaces(gs *gosnmp.GoSNMP, nodeID string) []InterfaceStat {
	var ifaces []InterfaceStat

	// Walk ifDescr
	descrResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.2")
	aliasResults := p.bulkWalk(gs, ".1.3.6.1.2.1.31.1.1.1.18") // ifAlias (optional)
	inResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.10")       // ifInOctets
	outResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.16")      // ifOutOctets
	inErrResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.14")    // ifInErrors
	outErrResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.20")   // ifOutErrors
	speedResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.5")     // ifSpeed
	operResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.8")      // ifOperStatus
	adminResults := p.bulkWalk(gs, ".1.3.6.1.2.1.2.2.1.7")     // ifAdminStatus

	// Build map of ifIndex -> values
	for _, v := range descrResults {
		idx := extractLastOID(v.Name)
		iface := InterfaceStat{Index: idx}
		if b, ok := v.Value.([]byte); ok {
			iface.Name = string(b)
		}
		ifaces = append(ifaces, iface)
	}

	applyWalkResults(ifaces, aliasResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		if b, ok := v.Value.([]byte); ok {
			is.Alias = string(b)
		}
	})
	applyWalkResults(ifaces, inResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		is.InOctets = gosnmp.ToBigInt(v.Value).Uint64()
	})
	applyWalkResults(ifaces, outResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		is.OutOctets = gosnmp.ToBigInt(v.Value).Uint64()
	})
	applyWalkResults(ifaces, inErrResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		is.InErrors = gosnmp.ToBigInt(v.Value).Uint64()
	})
	applyWalkResults(ifaces, outErrResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		is.OutErrors = gosnmp.ToBigInt(v.Value).Uint64()
	})
	applyWalkResults(ifaces, speedResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		is.Speed = gosnmp.ToBigInt(v.Value).Uint64()
	})
	applyWalkResults(ifaces, operResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		is.OperStatus = int(gosnmp.ToBigInt(v.Value).Int64())
	})
	applyWalkResults(ifaces, adminResults, func(is *InterfaceStat, v gosnmp.SnmpPDU) {
		is.AdminStatus = int(gosnmp.ToBigInt(v.Value).Int64())
	})

	return ifaces
}

func (p *Poller) bulkWalk(gs *gosnmp.GoSNMP, oid string) []gosnmp.SnmpPDU {
	var results []gosnmp.SnmpPDU
	err := gs.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	})
	if err != nil {
		log.Printf("[snmp] bulkwalk %s error: %v", oid, err)
	}
	return results
}

func extractLastOID(oid string) int {
	// Find the last component of the OID
	lastDot := -1
	for i := len(oid) - 1; i >= 0; i-- {
		if oid[i] == '.' {
			lastDot = i
			break
		}
	}
	if lastDot < 0 {
		return 0
	}
	var idx int
	fmt.Sscanf(oid[lastDot+1:], "%d", &idx)
	return idx
}

func applyWalkResults(ifaces []InterfaceStat, pdus []gosnmp.SnmpPDU, setter func(*InterfaceStat, gosnmp.SnmpPDU)) {
	for _, pdu := range pdus {
		idx := extractLastOID(pdu.Name)
		for i := range ifaces {
			if ifaces[i].Index == idx {
				setter(&ifaces[i], pdu)
				break
			}
		}
	}
}
