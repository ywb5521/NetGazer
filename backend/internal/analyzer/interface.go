package analyzer

import (
	"net"
	"strings"
)

type Analyzer interface {
	Name() string
	Limit() int
}

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type TCPAnalyzer interface {
	Analyzer
	NewTCP(TCPInfo, Logger) TCPStream
}

type TCPInfo struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
}

type TCPStream interface {
	Feed(rev, start, end bool, skip int, data []byte) (u *PropUpdate, done bool)
	Close(limited bool) *PropUpdate
}

type UDPAnalyzer interface {
	Analyzer
	NewUDP(UDPInfo, Logger) UDPStream
}

type UDPInfo struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
}

type UDPStream interface {
	Feed(rev bool, data []byte) (u *PropUpdate, done bool)
	Close(limited bool) *PropUpdate
}

type (
	PropMap         map[string]interface{}
	CombinedPropMap map[string]PropMap
)

func (m PropMap) Get(key string) interface{} {
	keys := strings.Split(key, ".")
	if len(keys) == 0 {
		return nil
	}
	var current interface{} = m
	for _, k := range keys {
		currentMap, ok := current.(PropMap)
		if !ok {
			return nil
		}
		current = currentMap[k]
	}
	return current
}

func (cm CombinedPropMap) Get(an string, key string) interface{} {
	m, ok := cm[an]
	if !ok {
		return nil
	}
	return m.Get(key)
}

type PropUpdateType int

const (
	PropUpdateNone PropUpdateType = iota
	PropUpdateMerge
	PropUpdateReplace
	PropUpdateDelete
)

type PropUpdate struct {
	Type PropUpdateType
	M    PropMap
}
