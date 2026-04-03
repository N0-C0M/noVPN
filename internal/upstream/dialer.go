package upstream

import (
	"context"
	"errors"
	"net"
	"time"

	"novpn/pkg/model"
)

type Dialer interface {
	DialTCP(ctx context.Context, target model.TargetInfo) (net.Conn, error)
	DialUDP(ctx context.Context, target model.TargetInfo) (*net.UDPConn, error)
}

type DirectDialer struct {
	timeout time.Duration
}

func NewDirectDialer(timeout time.Duration) *DirectDialer {
	return &DirectDialer{timeout: timeout}
}

func (d *DirectDialer) DialTCP(ctx context.Context, target model.TargetInfo) (net.Conn, error) {
	if d.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.timeout)
		defer cancel()
	}

	return (&net.Dialer{}).DialContext(ctx, "tcp", target.UpstreamAddr)
}

func (d *DirectDialer) DialUDP(ctx context.Context, target model.TargetInfo) (*net.UDPConn, error) {
	if d.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.timeout)
		defer cancel()
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", target.UpstreamAddr)
	if err != nil {
		return nil, err
	}

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		_ = conn.Close()
		return nil, errors.New("unexpected UDP connection type")
	}

	return udpConn, nil
}
