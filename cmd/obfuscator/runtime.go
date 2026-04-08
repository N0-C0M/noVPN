package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	socksVersion                = 0x05
	socksAuthNone               = 0x00
	socksAuthUsernamePassword   = 0x02
	socksAuthNoAcceptableMethod = 0xff

	socksCommandConnect = 0x01

	socksReplySucceeded             = 0x00
	socksReplyGeneralFailure        = 0x01
	socksReplyCommandNotSupported   = 0x07
	socksReplyAddressTypeNotSupport = 0x08
)

type socksRequest struct {
	host         string
	port         int
	addressBytes []byte
}

type relayPlan struct {
	rng                *rand.Rand
	startupDelayMax    time.Duration
	interChunkDelayMax time.Duration
	idlePauseMin       time.Duration
	idlePauseMax       time.Duration
	burstBudgetMin     int
	burstBudgetMax     int
	initialWindowBytes int
	warmChunkMin       int
	warmChunkMax       int
	steadyChunkMin     int
	steadyChunkMax     int
}

type relayStats struct {
	uploadBytes   int64
	downloadBytes int64
}

func runProxyRuntime(ctx context.Context, cfg *config) error {
	listener, err := net.Listen("tcp", cfg.Listen.address())
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Listen.address(), err)
	}
	defer listener.Close()

	log.Printf("proxy_listen=%s upstream=%s", cfg.Listen.address(), cfg.Upstream.address())

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	var sessionID uint64
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}

		id := atomic.AddUint64(&sessionID, 1)
		go func(sessionConn net.Conn, currentID uint64) {
			if err := handleClientSession(ctx, cfg, sessionConn, currentID); err != nil && ctx.Err() == nil {
				log.Printf("session=%d client=%s error=%v", currentID, sessionConn.RemoteAddr(), err)
			}
		}(conn, id)
	}
}

func handleClientSession(ctx context.Context, cfg *config, clientConn net.Conn, sessionID uint64) error {
	defer clientConn.Close()

	if tcpConn, ok := clientConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}

	request, err := acceptClientConnect(clientConn, cfg.Listen)
	if err != nil {
		return err
	}

	destination := request.destination()
	upstreamConn, err := dialUpstreamConnect(ctx, cfg.Upstream, request)
	if err != nil {
		_ = writeSocksReply(clientConn, socksReplyGeneralFailure)
		return fmt.Errorf("dial upstream %s via %s: %w", destination, cfg.Upstream.address(), err)
	}
	defer upstreamConn.Close()

	if tcpConn, ok := upstreamConn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	if err := writeSocksReply(clientConn, socksReplySucceeded); err != nil {
		return fmt.Errorf("respond to client: %w", err)
	}

	uplinkPlan := newRelayPlan(cfg, destination, "uplink", sessionID)
	downlinkPlan := newRelayPlan(cfg, destination, "downlink", sessionID)

	log.Printf(
		"session=%d established client=%s destination=%s upstream=%s",
		sessionID,
		clientConn.RemoteAddr(),
		destination,
		cfg.Upstream.address(),
	)

	stats, err := relayBidirectional(ctx, clientConn, upstreamConn, uplinkPlan, downlinkPlan)
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf(
			"relay destination=%s upload=%d download=%d: %w",
			destination,
			stats.uploadBytes,
			stats.downloadBytes,
			err,
		)
	}

	log.Printf(
		"session=%d closed destination=%s upload_bytes=%d download_bytes=%d",
		sessionID,
		destination,
		stats.uploadBytes,
		stats.downloadBytes,
	)
	return nil
}

func acceptClientConnect(conn net.Conn, endpoint socksEndpoint) (socksRequest, error) {
	methods, err := readClientGreeting(conn)
	if err != nil {
		return socksRequest{}, fmt.Errorf("read greeting: %w", err)
	}

	selectedMethod, err := selectAuthMethod(methods, endpoint)
	if err != nil {
		_, _ = conn.Write([]byte{socksVersion, socksAuthNoAcceptableMethod})
		return socksRequest{}, err
	}
	if _, err := conn.Write([]byte{socksVersion, selectedMethod}); err != nil {
		return socksRequest{}, fmt.Errorf("write greeting reply: %w", err)
	}

	if selectedMethod == socksAuthUsernamePassword {
		if err := authenticateClient(conn, endpoint); err != nil {
			return socksRequest{}, err
		}
	}

	request, replyCode, err := readSocksConnectRequest(conn)
	if err != nil {
		if replyCode != 0 {
			_ = writeSocksReply(conn, replyCode)
		}
		return socksRequest{}, err
	}
	return request, nil
}

func readClientGreeting(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	if header[0] != socksVersion {
		return nil, fmt.Errorf("unexpected SOCKS version %d", header[0])
	}

	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return nil, err
	}
	return methods, nil
}

func selectAuthMethod(methods []byte, endpoint socksEndpoint) (byte, error) {
	if endpoint.requiresAuth() {
		if containsByte(methods, socksAuthUsernamePassword) {
			return socksAuthUsernamePassword, nil
		}
		return 0, errors.New("client does not support username/password authentication")
	}
	if containsByte(methods, socksAuthNone) {
		return socksAuthNone, nil
	}
	return 0, errors.New("client does not support no-auth SOCKS authentication")
}

func authenticateClient(conn net.Conn, endpoint socksEndpoint) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read auth header: %w", err)
	}
	if header[0] != 0x01 {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return fmt.Errorf("unexpected auth version %d", header[0])
	}

	username := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, username); err != nil {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return fmt.Errorf("read username: %w", err)
	}

	passwordLength := make([]byte, 1)
	if _, err := io.ReadFull(conn, passwordLength); err != nil {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return fmt.Errorf("read password length: %w", err)
	}

	password := make([]byte, int(passwordLength[0]))
	if _, err := io.ReadFull(conn, password); err != nil {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return fmt.Errorf("read password: %w", err)
	}

	if string(username) != endpoint.Username || string(password) != endpoint.Password {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return errors.New("client provided invalid SOCKS credentials")
	}

	if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
		return fmt.Errorf("write auth reply: %w", err)
	}
	return nil
}

func readSocksConnectRequest(conn net.Conn) (socksRequest, byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return socksRequest{}, 0, fmt.Errorf("read connect request header: %w", err)
	}
	if header[0] != socksVersion {
		return socksRequest{}, 0, fmt.Errorf("unexpected connect request version %d", header[0])
	}
	if header[1] != socksCommandConnect {
		return socksRequest{}, socksReplyCommandNotSupported, fmt.Errorf("unsupported command %d", header[1])
	}

	addressBytes, host, err := readSocksAddress(conn, header[3])
	if err != nil {
		return socksRequest{}, socksReplyAddressTypeNotSupport, err
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return socksRequest{}, 0, fmt.Errorf("read connect port: %w", err)
	}
	port := int(binary.BigEndian.Uint16(portBytes))

	return socksRequest{
		host:         host,
		port:         port,
		addressBytes: append(addressBytes, portBytes...),
	}, 0, nil
}

func readSocksAddress(conn net.Conn, atyp byte) ([]byte, string, error) {
	switch atyp {
	case 0x01:
		address := make([]byte, 4)
		if _, err := io.ReadFull(conn, address); err != nil {
			return nil, "", fmt.Errorf("read IPv4 address: %w", err)
		}
		return append([]byte{atyp}, address...), net.IP(address).String(), nil
	case 0x03:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return nil, "", fmt.Errorf("read domain length: %w", err)
		}
		domain := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, domain); err != nil {
			return nil, "", fmt.Errorf("read domain: %w", err)
		}
		return append([]byte{atyp, length[0]}, domain...), string(domain), nil
	case 0x04:
		address := make([]byte, 16)
		if _, err := io.ReadFull(conn, address); err != nil {
			return nil, "", fmt.Errorf("read IPv6 address: %w", err)
		}
		return append([]byte{atyp}, address...), net.IP(address).String(), nil
	default:
		return nil, "", fmt.Errorf("unsupported address type %d", atyp)
	}
}

func dialUpstreamConnect(ctx context.Context, endpoint socksEndpoint, request socksRequest) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint.address())
	if err != nil {
		return nil, err
	}

	if err := negotiateUpstreamConnect(conn, endpoint, request); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func negotiateUpstreamConnect(conn net.Conn, endpoint socksEndpoint, request socksRequest) error {
	if endpoint.requiresAuth() {
		if _, err := conn.Write([]byte{socksVersion, 0x01, socksAuthUsernamePassword}); err != nil {
			return fmt.Errorf("write upstream greeting: %w", err)
		}
	} else {
		if _, err := conn.Write([]byte{socksVersion, 0x01, socksAuthNone}); err != nil {
			return fmt.Errorf("write upstream greeting: %w", err)
		}
	}

	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("read upstream greeting reply: %w", err)
	}
	if reply[0] != socksVersion {
		return fmt.Errorf("unexpected upstream SOCKS version %d", reply[0])
	}
	switch reply[1] {
	case socksAuthNone:
		// nothing to do
	case socksAuthUsernamePassword:
		if err := writeUpstreamCredentials(conn, endpoint); err != nil {
			return err
		}
	default:
		return fmt.Errorf("upstream rejected auth method %d", reply[1])
	}

	connectRequest := append([]byte{socksVersion, socksCommandConnect, 0x00}, request.addressBytes...)
	if _, err := conn.Write(connectRequest); err != nil {
		return fmt.Errorf("write upstream connect: %w", err)
	}

	connectReply := make([]byte, 4)
	if _, err := io.ReadFull(conn, connectReply); err != nil {
		return fmt.Errorf("read upstream connect reply: %w", err)
	}
	if connectReply[0] != socksVersion {
		return fmt.Errorf("unexpected upstream connect reply version %d", connectReply[0])
	}
	if connectReply[1] != socksReplySucceeded {
		return fmt.Errorf("upstream connect failed with code %d", connectReply[1])
	}

	if _, _, err := readSocksAddress(conn, connectReply[3]); err != nil {
		return fmt.Errorf("read upstream bind address: %w", err)
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return fmt.Errorf("read upstream bind port: %w", err)
	}
	return nil
}

func writeUpstreamCredentials(conn net.Conn, endpoint socksEndpoint) error {
	if !endpoint.requiresAuth() {
		return errors.New("upstream requested authentication but no credentials were configured")
	}
	username := []byte(endpoint.Username)
	password := []byte(endpoint.Password)
	if len(username) > 255 || len(password) > 255 {
		return errors.New("upstream SOCKS credentials are too long")
	}

	payload := make([]byte, 0, 3+len(username)+len(password))
	payload = append(payload, 0x01, byte(len(username)))
	payload = append(payload, username...)
	payload = append(payload, byte(len(password)))
	payload = append(payload, password...)
	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write upstream auth payload: %w", err)
	}

	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("read upstream auth reply: %w", err)
	}
	if reply[0] != 0x01 || reply[1] != 0x00 {
		return fmt.Errorf("upstream rejected credentials with code %d", reply[1])
	}
	return nil
}

func relayBidirectional(
	ctx context.Context,
	clientConn net.Conn,
	upstreamConn net.Conn,
	uplink relayPlan,
	downlink relayPlan,
) (relayStats, error) {
	var stats relayStats
	var firstErr error
	var errMu sync.Mutex
	var once sync.Once
	done := make(chan struct{})

	interrupt := func(err error) {
		if err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			return
		}
		once.Do(func() {
			errMu.Lock()
			firstErr = err
			errMu.Unlock()
			_ = clientConn.Close()
			_ = upstreamConn.Close()
		})
	}

	go func() {
		select {
		case <-ctx.Done():
			_ = clientConn.Close()
			_ = upstreamConn.Close()
		case <-done:
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		written, err := uplink.relay(ctx, upstreamConn, clientConn)
		stats.uploadBytes = written
		if err == nil {
			_ = closeWrite(upstreamConn)
			return
		}
		interrupt(err)
	}()
	go func() {
		defer wg.Done()
		written, err := downlink.relay(ctx, clientConn, upstreamConn)
		stats.downloadBytes = written
		if err == nil {
			_ = closeWrite(clientConn)
			return
		}
		interrupt(err)
	}()
	wg.Wait()
	close(done)

	errMu.Lock()
	defer errMu.Unlock()
	if firstErr != nil {
		return stats, firstErr
	}
	if ctx.Err() != nil {
		return stats, ctx.Err()
	}
	return stats, nil
}

func newRelayPlan(cfg *config, destination string, direction string, sessionID uint64) relayPlan {
	density := paddingDensity(cfg.PatternTuning.PaddingProfile)
	isDownlink := direction == "downlink"
	rng := seededRand(cfg, destination, direction, sessionID)

	warmChunkMin := clampInt(cfg.PatternTuning.PaddingMinBytes, 96, 768)
	warmChunkMax := clampInt(cfg.PatternTuning.PaddingMaxBytes, warmChunkMin+32, 2048)
	steadyChunkMin := clampInt(warmChunkMin*2, 256, 2048)
	steadyChunkMax := clampInt(warmChunkMax*3, steadyChunkMin+64, 8192)
	burstBudgetMin := clampInt(cfg.PatternTuning.PaddingMinBytes*(28-density*5), 10*1024, 64*1024)
	burstBudgetMax := clampInt(cfg.PatternTuning.PaddingMaxBytes*(40-density*4), burstBudgetMin+2048, 128*1024)
	startupDelayMax := clampDuration(
		time.Duration(cfg.PatternTuning.JitterWindowMs/maxInt(6, 12-density*2))*time.Millisecond,
		0,
		120*time.Millisecond,
	)
	interChunkDelayMax := clampDuration(
		time.Duration(cfg.PatternTuning.JitterWindowMs/maxInt(8, 26-density*4))*time.Millisecond,
		0,
		18*time.Millisecond,
	)
	idlePauseMin := scalePause(cfg.PatternTuning.IdleGapMinMs, 140-density*15, 6*time.Millisecond, 24*time.Millisecond)
	idlePauseMax := scalePause(cfg.PatternTuning.IdleGapMaxMs, 110-density*10, 10*time.Millisecond, 40*time.Millisecond)
	initialWindowBytes := clampInt(cfg.PatternTuning.PaddingMaxBytes*(24-density*4), 12*1024, 96*1024)

	if isDownlink {
		warmChunkMin = clampInt(warmChunkMin*2, 192, 1536)
		warmChunkMax = clampInt(warmChunkMax*2, warmChunkMin+64, 4096)
		steadyChunkMin = clampInt(steadyChunkMin*2, 512, 4096)
		steadyChunkMax = clampInt(steadyChunkMax*2, steadyChunkMin+128, 12*1024)
		burstBudgetMin = clampInt(burstBudgetMin*2, 20*1024, 128*1024)
		burstBudgetMax = clampInt(burstBudgetMax*2, burstBudgetMin+4096, 256*1024)
		startupDelayMax /= 2
		interChunkDelayMax /= 2
		idlePauseMin /= 2
		idlePauseMax /= 2
		initialWindowBytes = clampInt(initialWindowBytes*2, 24*1024, 192*1024)
	}

	return relayPlan{
		rng:                rng,
		startupDelayMax:    startupDelayMax,
		interChunkDelayMax: interChunkDelayMax,
		idlePauseMin:       idlePauseMin,
		idlePauseMax:       idlePauseMax,
		burstBudgetMin:     burstBudgetMin,
		burstBudgetMax:     burstBudgetMax,
		initialWindowBytes: initialWindowBytes,
		warmChunkMin:       warmChunkMin,
		warmChunkMax:       warmChunkMax,
		steadyChunkMin:     steadyChunkMin,
		steadyChunkMax:     steadyChunkMax,
	}
}

func (plan relayPlan) relay(ctx context.Context, dst net.Conn, src net.Conn) (int64, error) {
	if plan.startupDelayMax > 0 {
		if err := sleepWithContext(ctx, plan.randomDuration(0, plan.startupDelayMax)); err != nil {
			return 0, err
		}
	}

	buffer := make([]byte, 32*1024)
	var totalWritten int64
	bytesUntilPause := plan.randomInt(plan.burstBudgetMin, plan.burstBudgetMax)

	for {
		readBytes, err := src.Read(buffer)
		if readBytes > 0 {
			pending := buffer[:readBytes]
			for len(pending) > 0 {
				chunkSize := plan.nextChunkSize(int(totalWritten), len(pending))
				if totalWritten > 0 && plan.interChunkDelayMax > 0 {
					if err := sleepWithContext(ctx, plan.randomDuration(0, plan.interChunkDelayMax)); err != nil {
						return totalWritten, err
					}
				}
				if err := writeAll(dst, pending[:chunkSize]); err != nil {
					return totalWritten, err
				}
				pending = pending[chunkSize:]
				totalWritten += int64(chunkSize)
				bytesUntilPause -= chunkSize
				if bytesUntilPause <= 0 && plan.idlePauseMax > 0 {
					if err := sleepWithContext(ctx, plan.randomDuration(plan.idlePauseMin, plan.idlePauseMax)); err != nil {
						return totalWritten, err
					}
					bytesUntilPause = plan.randomInt(plan.burstBudgetMin, plan.burstBudgetMax)
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return totalWritten, nil
			}
			return totalWritten, err
		}
	}
}

func (plan relayPlan) nextChunkSize(totalWritten int, available int) int {
	chunkMin := plan.warmChunkMin
	chunkMax := plan.warmChunkMax
	if totalWritten >= plan.initialWindowBytes {
		chunkMin = plan.steadyChunkMin
		chunkMax = plan.steadyChunkMax
	}
	chunkSize := plan.randomInt(chunkMin, chunkMax)
	if chunkSize < 1 {
		chunkSize = 1
	}
	if chunkSize > available {
		chunkSize = available
	}
	return chunkSize
}

func (plan relayPlan) randomInt(minValue int, maxValue int) int {
	if maxValue <= minValue {
		return minValue
	}
	return minValue + plan.rng.Intn(maxValue-minValue+1)
}

func (plan relayPlan) randomDuration(minValue time.Duration, maxValue time.Duration) time.Duration {
	if maxValue <= minValue {
		return minValue
	}
	span := maxValue - minValue
	return minValue + time.Duration(plan.rng.Int63n(int64(span)+1))
}

func paddingDensity(profile string) int {
	switch {
	case strings.HasSuffix(profile, "dense"):
		return 3
	case strings.HasSuffix(profile, "mixed"):
		return 2
	default:
		return 1
	}
}

func seededRand(cfg *config, destination string, direction string, sessionID uint64) *rand.Rand {
	sum := sha256.Sum256([]byte(
		fmt.Sprintf(
			"%s|%s|%s|%d|%s|%d",
			cfg.Seed,
			cfg.Session.Nonce,
			destination,
			cfg.Session.RotationBucket,
			direction,
			sessionID,
		),
	))
	seed := int64(binary.LittleEndian.Uint64(sum[:8]) & 0x7fffffffffffffff)
	if seed == 0 {
		seed = 1
	}
	return rand.New(rand.NewSource(seed))
}

func writeAll(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		written, err := writer.Write(payload)
		if err != nil {
			return err
		}
		payload = payload[written:]
	}
	return nil
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func writeSocksReply(conn net.Conn, replyCode byte) error {
	_, err := conn.Write([]byte{socksVersion, replyCode, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	return err
}

func closeWrite(conn net.Conn) error {
	type closeWriter interface {
		CloseWrite() error
	}
	if value, ok := conn.(closeWriter); ok {
		return value.CloseWrite()
	}
	return nil
}

func containsByte(values []byte, target byte) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampDuration(value time.Duration, minValue time.Duration, maxValue time.Duration) time.Duration {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func scalePause(sourceMs int, divisor int, minValue time.Duration, maxValue time.Duration) time.Duration {
	if divisor <= 0 {
		divisor = 1
	}
	return clampDuration(time.Duration(sourceMs/divisor)*time.Millisecond, minValue, maxValue)
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func (endpoint socksEndpoint) address() string {
	host := endpoint.Address
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(endpoint.Port))
}

func (request socksRequest) destination() string {
	return net.JoinHostPort(request.host, strconv.Itoa(request.port))
}
