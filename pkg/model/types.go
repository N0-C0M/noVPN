package model

import (
	"net"
	"time"
)

type ConnMetadata struct {
	ClientAddr   net.Addr
	ListenerName string
	ReceivedAt   time.Time
}

type PacketMetadata struct {
	ClientAddr   net.Addr
	ListenerName string
	Size         int
	ReceivedAt   time.Time
}

type Identity struct {
	Subject  string
	SourceIP net.IP
	TokenID  string
	MTLS     bool
	Labels   map[string]string
}

type TargetInfo struct {
	Network      string
	UpstreamAddr string
	Profile      string
}

type Decision struct {
	Allowed bool
	Reason  string
}
