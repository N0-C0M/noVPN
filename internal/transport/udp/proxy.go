package udp

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

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
	cfg      config.UDPListenerConfig
	deps     Dependencies
	listener net.PacketConn
	closing  atomic.Bool
	done     chan struct{}
	wg       sync.WaitGroup

	mu       sync.RWMutex
	sessions map[string]*session
}

type session struct {
	key        string
	clientAddr net.Addr
	identity   model.Identity
	upstream   *net.UDPConn
	lastSeen   atomic.Int64
	closed     atomic.Bool
}

func NewProxy(cfg config.UDPListenerConfig, deps Dependencies) *Proxy {
	return &Proxy{
		cfg:      cfg,
		deps:     deps,
		done:     make(chan struct{}),
		sessions: make(map[string]*session),
	}
}

func (p *Proxy) Name() string {
	return p.cfg.Name
}

func (p *Proxy) Start() error {
	listener, err := net.ListenPacket("udp", p.cfg.ListenAddr)
	if err != nil {
		return err
	}

	p.listener = listener
	p.wg.Add(2)
	go p.readLoop()
	go p.cleanupLoop()

	p.deps.Logger.Info("udp listener started", "addr", p.cfg.ListenAddr, "upstream", p.cfg.UpstreamAddr)
	return nil
}

func (p *Proxy) Shutdown(ctx context.Context) error {
	if p.closing.CompareAndSwap(false, true) {
		close(p.done)
		if p.listener != nil {
			_ = p.listener.Close()
		}
	}

	p.closeAllSessions()

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

func (p *Proxy) readLoop() {
	defer p.wg.Done()

	buffer := make([]byte, 64*1024)
	for {
		n, clientAddr, err := p.listener.ReadFrom(buffer)
		if err != nil {
			if p.closing.Load() || errors.Is(err, net.ErrClosed) {
				return
			}
			p.deps.Logger.Error("udp read failed", "error", err)
			continue
		}

		if p.cfg.Limits.MaxPacketSize > 0 && n > p.cfg.Limits.MaxPacketSize {
			p.deps.Metrics.UDPPacketsDroppedTotal.Inc()
			p.deps.Logger.Warn("udp packet dropped: size limit exceeded", "client", clientAddr.String(), "size", n)
			continue
		}

		packet := make([]byte, n)
		copy(packet, buffer[:n])

		p.handlePacket(clientAddr, packet)
	}
}

func (p *Proxy) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.cfg.Session.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.evictIdleSessions()
		case <-p.done:
			return
		}
	}
}

func (p *Proxy) handlePacket(clientAddr net.Addr, packet []byte) {
	key := remoteKey(clientAddr)
	if !p.deps.Limiter.AllowPacket(key, len(packet)) {
		p.deps.Metrics.UDPRejectedTotal.Inc()
		p.deps.Metrics.UDPPacketsDroppedTotal.Inc()
		return
	}

	meta := model.PacketMetadata{
		ClientAddr:   clientAddr,
		ListenerName: p.cfg.Name,
		Size:         len(packet),
		ReceivedAt:   time.Now(),
	}

	identity, err := p.deps.Auth.AuthenticateUDP(context.Background(), meta)
	if err != nil {
		p.deps.Metrics.AuthFailuresTotal.Inc()
		p.deps.Metrics.UDPRejectedTotal.Inc()
		p.deps.Metrics.UDPPacketsDroppedTotal.Inc()
		p.deps.Logger.Warn("udp auth failed", "client", clientAddr.String(), "error", err)
		return
	}

	target := model.TargetInfo{
		Network:      "udp",
		UpstreamAddr: p.cfg.UpstreamAddr,
		Profile:      p.cfg.Name,
	}

	decision, err := p.deps.ACL.Allow(identity, target)
	if err != nil || !decision.Allowed {
		p.deps.Metrics.ACLDeniesTotal.Inc()
		p.deps.Metrics.UDPRejectedTotal.Inc()
		p.deps.Metrics.UDPPacketsDroppedTotal.Inc()
		p.deps.Logger.Warn("udp acl denied", "client", clientAddr.String(), "reason", decision.Reason, "error", err)
		return
	}

	sess, err := p.getOrCreateSession(key, clientAddr, identity, target)
	if err != nil {
		p.deps.Metrics.UDPRejectedTotal.Inc()
		p.deps.Metrics.UDPPacketsDroppedTotal.Inc()
		p.deps.Logger.Warn("udp session create failed", "client", clientAddr.String(), "error", err)
		return
	}

	sess.touch()
	if p.cfg.Session.IdleTTL > 0 {
		_ = sess.upstream.SetWriteDeadline(time.Now().Add(p.cfg.Session.IdleTTL))
	}

	if _, err := sess.upstream.Write(packet); err != nil {
		p.deps.Metrics.UDPPacketsDroppedTotal.Inc()
		p.deps.Logger.Warn("udp upstream write failed", "client", clientAddr.String(), "error", err)
		p.closeSession(sess.key)
		return
	}

	p.deps.Metrics.UDPPacketsInTotal.Inc()
}

func (p *Proxy) getOrCreateSession(key string, clientAddr net.Addr, identity model.Identity, target model.TargetInfo) (*session, error) {
	p.mu.RLock()
	existing := p.sessions[key]
	p.mu.RUnlock()
	if existing != nil {
		return existing, nil
	}

	upstreamConn, err := p.deps.Dialer.DialUDP(context.Background(), target)
	if err != nil {
		return nil, err
	}

	sess := &session{
		key:        key,
		clientAddr: clientAddr,
		identity:   identity,
		upstream:   upstreamConn,
	}
	sess.touch()

	p.mu.Lock()
	if current := p.sessions[key]; current != nil {
		p.mu.Unlock()
		_ = upstreamConn.Close()
		return current, nil
	}
	if p.cfg.Session.MaxSessions > 0 && len(p.sessions) >= p.cfg.Session.MaxSessions {
		p.mu.Unlock()
		_ = upstreamConn.Close()
		return nil, errors.New("udp session limit reached")
	}
	p.sessions[key] = sess
	p.mu.Unlock()

	p.deps.Metrics.UDPSessionsActive.Inc()
	p.wg.Add(1)
	go p.readFromUpstream(sess)

	return sess, nil
}

func (p *Proxy) readFromUpstream(sess *session) {
	defer p.wg.Done()

	buffer := make([]byte, 64*1024)
	for {
		deadline := 5 * time.Second
		if p.cfg.Session.IdleTTL > 0 && p.cfg.Session.IdleTTL < deadline {
			deadline = p.cfg.Session.IdleTTL
		}
		_ = sess.upstream.SetReadDeadline(time.Now().Add(deadline))

		n, err := sess.upstream.Read(buffer)
		if n > 0 {
			sess.touch()
			if _, writeErr := p.listener.WriteTo(buffer[:n], sess.clientAddr); writeErr != nil {
				if !p.closing.Load() && !errors.Is(writeErr, net.ErrClosed) {
					p.deps.Logger.Warn("udp client write failed", "client", sess.clientAddr.String(), "error", writeErr)
				}
				p.closeSession(sess.key)
				return
			}
			p.deps.Metrics.UDPPacketsOutTotal.Inc()
		}

		if err != nil {
			if p.closing.Load() || errors.Is(err, net.ErrClosed) {
				p.closeSession(sess.key)
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if p.sessionIdle(sess) {
					p.closeSession(sess.key)
					return
				}
				continue
			}
			p.deps.Logger.Warn("udp upstream read failed", "client", sess.clientAddr.String(), "error", err)
			p.closeSession(sess.key)
			return
		}
	}
}

func (p *Proxy) evictIdleSessions() {
	p.mu.RLock()
	keys := make([]string, 0, len(p.sessions))
	for key, sess := range p.sessions {
		if p.sessionIdle(sess) {
			keys = append(keys, key)
		}
	}
	p.mu.RUnlock()

	for _, key := range keys {
		p.closeSession(key)
	}
}

func (p *Proxy) sessionIdle(sess *session) bool {
	if p.cfg.Session.IdleTTL <= 0 {
		return false
	}
	lastSeen := time.Unix(0, sess.lastSeen.Load())
	return time.Since(lastSeen) > p.cfg.Session.IdleTTL
}

func (p *Proxy) closeAllSessions() {
	p.mu.RLock()
	keys := make([]string, 0, len(p.sessions))
	for key := range p.sessions {
		keys = append(keys, key)
	}
	p.mu.RUnlock()

	for _, key := range keys {
		p.closeSession(key)
	}
}

func (p *Proxy) closeSession(key string) {
	p.mu.Lock()
	sess := p.sessions[key]
	if sess != nil {
		delete(p.sessions, key)
	}
	p.mu.Unlock()

	if sess == nil {
		return
	}
	if !sess.closed.CompareAndSwap(false, true) {
		return
	}

	_ = sess.upstream.Close()
	p.deps.Metrics.UDPSessionsActive.Dec()
}

func (s *session) touch() {
	s.lastSeen.Store(time.Now().UnixNano())
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
