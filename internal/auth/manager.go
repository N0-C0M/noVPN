package auth

import (
	"context"
	"net"

	"novpn/pkg/model"
)

type Manager interface {
	AuthenticateTCP(ctx context.Context, meta model.ConnMetadata) (model.Identity, error)
	AuthenticateUDP(ctx context.Context, meta model.PacketMetadata) (model.Identity, error)
}

type NoopManager struct{}

func (NoopManager) AuthenticateTCP(_ context.Context, meta model.ConnMetadata) (model.Identity, error) {
	return model.Identity{
		Subject:  "anonymous",
		SourceIP: sourceIP(meta.ClientAddr),
		Labels:   map[string]string{"auth_mode": "noop"},
	}, nil
}

func (NoopManager) AuthenticateUDP(_ context.Context, meta model.PacketMetadata) (model.Identity, error) {
	return model.Identity{
		Subject:  "anonymous",
		SourceIP: sourceIP(meta.ClientAddr),
		Labels:   map[string]string{"auth_mode": "noop"},
	}, nil
}

func sourceIP(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.TCPAddr:
		return value.IP
	case *net.UDPAddr:
		return value.IP
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return nil
		}
		return net.ParseIP(host)
	}
}
