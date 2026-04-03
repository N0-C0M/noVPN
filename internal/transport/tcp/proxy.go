package tcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"novpn/internal/acl"
	"novpn/internal/auth"
	"novpn/internal/config"
	"novpn/internal/observability"
	"novpn/internal/ratelimit"
	"novpn/internal/upstream"
	"novpn/pkg/model"
)

type Dependencies struct {
	Auth    auth.Manager
	ACL     acl.Evaluator
	Limiter ratelimit.Limiter
	Dialer  upstream.Dialer
	Metrics *observability.Metrics
	Logger  *slog.Logger
}

type Proxy struct {
	cfg      config.TCPListenerConfig
	deps     Dependencies
	listener net.Listener
	closing  atomic.Bool
	wg       sync.WaitGroup
	bufPool  sync.Pool
}

func NewProxy(cfg config.TCPListenerConfig, deps Dependencies) *Proxy {
	return &Proxy{
		cfg:  cfg,
		deps: deps,
		bufPool: sync.Pool{
			New: func() any {
				return make([]byte, 32*1024)
			},
		},
	}
}

func (p *Proxy) Name() string {
	return p.cfg.Name
}

func (p *Proxy) Start() error {
	listener, err := net.Listen("tcp", p.cfg.ListenAddr)
	if err != nil {
		return err
	}

	p.listener = listener
	p.wg.Add(1)
	go p.acceptLoop()

	p.deps.Logger.Info("tcp listener started", "addr", p.cfg.ListenAddr, "upstream", p.cfg.UpstreamAddr)
	return nil
}

func (p *Proxy) Shutdown(ctx context.Context) error {
	if p.closing.CompareAndSwap(false, true) && p.listener != nil {
		_ = p.listener.Close()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.wg.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Proxy) acceptLoop() {
	defer p.wg.Done()

	for {
		conn, err := p.listener.Accept()
		if err != nil {
			if p.closing.Load() || errors.Is(err, net.ErrClosed) {
				return
			}
			p.deps.Metrics.TCPAcceptErrorsTotal.Inc()
			p.deps.Logger.Error("tcp accept failed", "error", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.handleConn(conn)
		}()
	}
}

func (p *Proxy) handleConn(clientConn net.Conn) {
	startedAt := time.Now()
	p.deps.Metrics.TCPConnectionsActive.Inc()
	defer p.deps.Metrics.TCPConnectionsActive.Dec()
	defer clientConn.Close()

	if tcpConn, ok := clientConn.(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetNoDelay(true)
	}

	meta := model.ConnMetadata{
		ClientAddr:   clientConn.RemoteAddr(),
		ListenerName: p.cfg.Name,
		ReceivedAt:   startedAt,
	}

	identity, err := p.deps.Auth.AuthenticateTCP(context.Background(), meta)
	if err != nil {
		p.deps.Metrics.AuthFailuresTotal.Inc()
		p.deps.Metrics.TCPRejectedTotal.Inc()
		p.deps.Logger.Warn("tcp auth failed", "client", clientConn.RemoteAddr().String(), "error", err)
		return
	}

	target := model.TargetInfo{
		Network:      "tcp",
		UpstreamAddr: p.cfg.UpstreamAddr,
		Profile:      p.cfg.Name,
	}

	decision, err := p.deps.ACL.Allow(identity, target)
	if err != nil || !decision.Allowed {
		p.deps.Metrics.ACLDeniesTotal.Inc()
		p.deps.Metrics.TCPRejectedTotal.Inc()
		p.deps.Logger.Warn("tcp acl denied", "client", clientConn.RemoteAddr().String(), "reason", decision.Reason, "error", err)
		return
	}

	key := remoteKey(clientConn.RemoteAddr())
	if !p.deps.Limiter.AllowConnection(key) {
		p.deps.Metrics.TCPRejectedTotal.Inc()
		p.deps.Logger.Warn("tcp connection rejected by limiter", "client", clientConn.RemoteAddr().String())
		return
	}
	defer p.deps.Limiter.DoneConnection(key)

	dialCtx := context.Background()
	cancel := func() {}
	if p.cfg.Timeouts.Dial > 0 {
		dialCtx, cancel = context.WithTimeout(context.Background(), p.cfg.Timeouts.Dial)
	}
	defer cancel()

	upstreamConn, err := p.deps.Dialer.DialTCP(dialCtx, target)
	if err != nil {
		p.deps.Metrics.TCPUpstreamDialFailTotal.Inc()
		p.deps.Logger.Error("tcp upstream dial failed", "upstream", p.cfg.UpstreamAddr, "error", err)
		return
	}
	defer upstreamConn.Close()

	done := make(chan error, 2)
	go func() {
		done <- p.pipe(upstreamConn, clientConn, p.deps.Metrics.TCPBytesInTotal)
	}()
	go func() {
		done <- p.pipe(clientConn, upstreamConn, p.deps.Metrics.TCPBytesOutTotal)
	}()

	errIn := <-done
	errOut := <-done

	if errIn != nil && !isIgnorableNetErr(errIn) {
		p.deps.Logger.Debug("tcp client to upstream relay ended with error", "error", errIn)
	}
	if errOut != nil && !isIgnorableNetErr(errOut) {
		p.deps.Logger.Debug("tcp upstream to client relay ended with error", "error", errOut)
	}

	p.deps.Logger.Info(
		"tcp session closed",
		"client", clientConn.RemoteAddr().String(),
		"upstream", p.cfg.UpstreamAddr,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
}

func (p *Proxy) pipe(dst, src net.Conn, counter prometheus.Counter) error {
	buffer := p.bufPool.Get().([]byte)
	defer p.bufPool.Put(buffer)

	for {
		if p.cfg.Timeouts.Idle > 0 {
			_ = src.SetReadDeadline(time.Now().Add(p.cfg.Timeouts.Idle))
			_ = dst.SetWriteDeadline(time.Now().Add(p.cfg.Timeouts.Idle))
		}

		n, err := src.Read(buffer)
		if n > 0 {
			written := 0
			for written < n {
				chunk, writeErr := dst.Write(buffer[written:n])
				if chunk > 0 {
					written += chunk
					counter.Add(float64(chunk))
				}
				if writeErr != nil {
					return writeErr
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				closeWrite(dst)
				return nil
			}
			return err
		}
	}
}

func closeWrite(conn net.Conn) {
	type closeWriter interface {
		CloseWrite() error
	}

	if c, ok := conn.(closeWriter); ok {
		_ = c.CloseWrite()
	}
}

func isIgnorableNetErr(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func remoteKey(addr net.Addr) string {
	switch value := addr.(type) {
	case *net.TCPAddr:
		return value.IP.String()
	case *net.UDPAddr:
		return value.IP.String()
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return addr.String()
		}
		return host
	}
}
