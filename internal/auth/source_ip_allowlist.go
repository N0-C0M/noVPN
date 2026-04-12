package auth

import (
	"context"
	"fmt"
	"net"

	"novpn/pkg/model"
)

type SourceIPAllowlistManager struct {
	prefixes []*net.IPNet
}

func NewSourceIPAllowlistManager(cidrs []string) (SourceIPAllowlistManager, error) {
	prefixes := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		_, prefix, err := net.ParseCIDR(raw)
		if err != nil {
			return SourceIPAllowlistManager{}, fmt.Errorf("parse CIDR %q: %w", raw, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return SourceIPAllowlistManager{prefixes: prefixes}, nil
}

func (m SourceIPAllowlistManager) AuthenticateTCP(_ context.Context, meta model.ConnMetadata) (model.Identity, error) {
	return m.authenticate(meta.ClientAddr)
}

func (m SourceIPAllowlistManager) AuthenticateUDP(_ context.Context, meta model.PacketMetadata) (model.Identity, error) {
	return m.authenticate(meta.ClientAddr)
}

func (m SourceIPAllowlistManager) authenticate(addr net.Addr) (model.Identity, error) {
	ip := sourceIP(addr)
	if ip == nil {
		return model.Identity{}, fmt.Errorf("failed to extract source ip from %q", addr.String())
	}
	if !m.allowed(ip) {
		return model.Identity{}, fmt.Errorf("source ip %q is not in allowlist", ip.String())
	}
	return model.Identity{
		Subject:  ip.String(),
		SourceIP: ip,
		Labels: map[string]string{
			"auth_mode": "source_ip_allowlist",
		},
	}, nil
}

func (m SourceIPAllowlistManager) allowed(ip net.IP) bool {
	for _, prefix := range m.prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
