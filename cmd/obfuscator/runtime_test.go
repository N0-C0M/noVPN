package main

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestLoadSocksEndpointFromXrayConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "xray.json")
	payload := []byte(`{
  "inbounds": [
    {
      "protocol": "socks",
      "listen": "127.0.0.1",
      "port": 19090,
      "settings": {
        "auth": "password",
        "accounts": [
          {"user": "alpha", "pass": "beta"}
        ]
      }
    }
  ]
}`)
	if err := os.WriteFile(configPath, payload, 0o600); err != nil {
		t.Fatalf("write xray config: %v", err)
	}

	endpoint, err := loadSocksEndpointFromXrayConfig(configPath)
	if err != nil {
		t.Fatalf("loadSocksEndpointFromXrayConfig: %v", err)
	}

	if endpoint.Address != "127.0.0.1" || endpoint.Port != 19090 {
		t.Fatalf("unexpected endpoint: %+v", endpoint)
	}
	if endpoint.Username != "alpha" || endpoint.Password != "beta" {
		t.Fatalf("unexpected credentials: %+v", endpoint)
	}
}

func TestProxyRuntimeRelaysThroughUpstreamSocks(t *testing.T) {
	echoListener, echoAddress := startEchoServer(t)
	defer echoListener.Close()

	upstreamEndpoint, stopUpstream := startUpstreamSocksServer(t, socksEndpoint{
		Address:  "127.0.0.1",
		Username: "up-user",
		Password: "up-pass",
	})
	defer stopUpstream()

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy port: %v", err)
	}
	proxyAddress := proxyListener.Addr().String()
	proxyListener.Close()

	proxyHost, proxyPortText, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatalf("split proxy address: %v", err)
	}
	proxyPort, err := strconv.Atoi(proxyPortText)
	if err != nil {
		t.Fatalf("parse proxy port: %v", err)
	}

	cfg := &config{
		Mode: "client",
		Seed: "test-seed",
		Remote: remoteConfig{
			Address: "example.com",
			Port:    443,
		},
		Listen: socksEndpoint{
			Address: proxyHost,
			Port:    proxyPort,
		},
		Upstream: upstreamEndpoint,
		Session: sessionConfig{
			Nonce:          "abcd1234",
			RotationBucket: 42,
		},
		PatternTuning: patternTuningConfig{
			PaddingProfile:     "steady-light",
			JitterWindowMs:     40,
			PaddingMinBytes:    128,
			PaddingMaxBytes:    320,
			BurstIntervalMinMs: 800,
			BurstIntervalMaxMs: 1200,
			IdleGapMinMs:       1200,
			IdleGapMaxMs:       2200,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runProxyRuntime(ctx, cfg)
	}()

	waitForAddress(t, proxyAddress)

	clientConn := openClientSocksTunnel(t, proxyAddress, echoAddress)
	defer clientConn.Close()

	payload := []byte("android-obfuscator-live")
	if err := clientConn.SetDeadline(time.Now().Add(4 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	if _, err := clientConn.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(clientConn, reply); err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if string(reply) != string(payload) {
		t.Fatalf("unexpected reply: got %q want %q", string(reply), string(payload))
	}

	cancel()
	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Fatalf("runProxyRuntime: %v", runErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("proxy runtime did not stop after cancellation")
	}
}

func TestProxyRuntimeRelaysUDPAssociateThroughUpstreamSocks(t *testing.T) {
	upstreamEndpoint, stopUpstream := startUpstreamSocksServer(t, socksEndpoint{
		Address:  "127.0.0.1",
		Username: "up-user",
		Password: "up-pass",
	})
	defer stopUpstream()

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy port: %v", err)
	}
	proxyAddress := proxyListener.Addr().String()
	proxyListener.Close()

	proxyHost, proxyPortText, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatalf("split proxy address: %v", err)
	}
	proxyPort, err := strconv.Atoi(proxyPortText)
	if err != nil {
		t.Fatalf("parse proxy port: %v", err)
	}

	cfg := &config{
		Mode: "client",
		Seed: "test-seed",
		Remote: remoteConfig{
			Address: "example.com",
			Port:    443,
		},
		Listen: socksEndpoint{
			Address: proxyHost,
			Port:    proxyPort,
		},
		Upstream: upstreamEndpoint,
		Session: sessionConfig{
			Nonce:          "abcd1234",
			RotationBucket: 42,
		},
		PatternTuning: patternTuningConfig{
			PaddingProfile:     "steady-light",
			JitterWindowMs:     40,
			PaddingMinBytes:    128,
			PaddingMaxBytes:    320,
			BurstIntervalMinMs: 800,
			BurstIntervalMaxMs: 1200,
			IdleGapMinMs:       1200,
			IdleGapMaxMs:       2200,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- runProxyRuntime(ctx, cfg)
	}()

	waitForAddress(t, proxyAddress)

	controlConn, udpConn, relayAddr := openClientSocksUDPAssociate(t, proxyAddress)
	defer controlConn.Close()
	defer udpConn.Close()

	payload := []byte("udp-associate-probe")
	datagram := buildSocksUDPDatagram(t, "1.1.1.1:53", payload)
	if _, err := udpConn.WriteToUDP(datagram, relayAddr); err != nil {
		t.Fatalf("write udp datagram: %v", err)
	}

	reply := make([]byte, 64*1024)
	if err := udpConn.SetReadDeadline(time.Now().Add(4 * time.Second)); err != nil {
		t.Fatalf("set udp read deadline: %v", err)
	}
	n, _, err := udpConn.ReadFromUDP(reply)
	if err != nil {
		t.Fatalf("read udp datagram: %v", err)
	}
	if n != len(datagram) {
		t.Fatalf("unexpected udp datagram length: got %d want %d", n, len(datagram))
	}
	if string(reply[:n]) != string(datagram) {
		t.Fatalf("unexpected udp datagram content")
	}

	cancel()
	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Fatalf("runProxyRuntime: %v", runErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("proxy runtime did not stop after cancellation")
	}
}

func startEchoServer(t *testing.T) (net.Listener, string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen echo server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(active net.Conn) {
				defer active.Close()
				_, _ = io.Copy(active, active)
			}(conn)
		}
	}()

	return listener, listener.Addr().String()
}

func startUpstreamSocksServer(t *testing.T, endpoint socksEndpoint) (socksEndpoint, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen upstream socks server: %v", err)
	}

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split upstream address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse upstream port: %v", err)
	}
	endpoint.Address = host
	endpoint.Port = port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleUpstreamSocksConn(conn, endpoint)
		}
	}()

	stop := func() {
		_ = listener.Close()
	}
	return endpoint, stop
}

func handleUpstreamSocksConn(conn net.Conn, endpoint socksEndpoint) {
	defer conn.Close()

	methods, err := readClientGreeting(conn)
	if err != nil {
		return
	}

	selectedMethod, err := selectAuthMethod(methods, endpoint)
	if err != nil {
		_, _ = conn.Write([]byte{socksVersion, socksAuthNoAcceptableMethod})
		return
	}
	if _, err := conn.Write([]byte{socksVersion, selectedMethod}); err != nil {
		return
	}
	if selectedMethod == socksAuthUsernamePassword {
		if err := authenticateClient(conn, endpoint); err != nil {
			return
		}
	}

	request, replyCode, err := readSocksRequest(conn)
	if err != nil {
		if replyCode != 0 {
			_ = writeSocksReply(conn, replyCode)
		}
		return
	}

	switch request.command {
	case socksCommandConnect:
		targetConn, err := net.Dial("tcp", request.destination())
		if err != nil {
			_ = writeSocksReply(conn, socksReplyGeneralFailure)
			return
		}
		defer targetConn.Close()

		if err := writeSocksReply(conn, socksReplySucceeded); err != nil {
			return
		}

		done := make(chan struct{}, 2)
		go func() {
			_, _ = io.Copy(targetConn, conn)
			_ = closeWrite(targetConn)
			done <- struct{}{}
		}()
		go func() {
			_, _ = io.Copy(conn, targetConn)
			_ = closeWrite(conn)
			done <- struct{}{}
		}()
		<-done
		<-done
	case socksCommandUDPAssociate:
		relayConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
		if err != nil {
			_ = writeSocksReply(conn, socksReplyGeneralFailure)
			return
		}
		defer relayConn.Close()

		replyAddr := advertisedUDPRelayAddress(conn.LocalAddr(), relayConn.LocalAddr().(*net.UDPAddr))
		if err := writeSocksReplyWithBindAddress(conn, socksReplySucceeded, replyAddr); err != nil {
			return
		}

		stop := make(chan struct{})
		errCh := make(chan error, 1)
		go func() {
			errCh <- handleUpstreamUDPRelay(relayConn, stop)
		}()

		_, _ = io.Copy(io.Discard, conn)
		close(stop)
		_ = relayConn.Close()
		<-errCh
	default:
		_ = writeSocksReply(conn, socksReplyCommandNotSupported)
	}
}

func handleUpstreamUDPRelay(relayConn *net.UDPConn, stop <-chan struct{}) error {
	buffer := make([]byte, 64*1024)
	for {
		_ = relayConn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, addr, err := relayConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-stop:
					return nil
				default:
					continue
				}
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if n == 0 {
			continue
		}
		if _, err := relayConn.WriteToUDP(buffer[:n], addr); err != nil {
			return err
		}
	}
}

func openClientSocksTunnel(t *testing.T, proxyAddress string, destination string) net.Conn {
	t.Helper()

	conn, err := net.Dial("tcp", proxyAddress)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}

	if _, err := conn.Write([]byte{socksVersion, 0x01, socksAuthNone}); err != nil {
		t.Fatalf("write client greeting: %v", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read client greeting reply: %v", err)
	}
	if reply[0] != socksVersion || reply[1] != socksAuthNone {
		t.Fatalf("unexpected greeting reply: %v", reply)
	}

	host, portText, err := net.SplitHostPort(destination)
	if err != nil {
		t.Fatalf("split destination: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse destination port: %v", err)
	}

	addressBytes := buildAddressBytes(t, host)
	request := append([]byte{socksVersion, socksCommandConnect, 0x00}, addressBytes...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	request = append(request, portBytes...)
	if _, err := conn.Write(request); err != nil {
		t.Fatalf("write connect request: %v", err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read connect reply header: %v", err)
	}
	if header[0] != socksVersion || header[1] != socksReplySucceeded {
		t.Fatalf("unexpected connect reply header: %v", header)
	}
	if _, _, err := readSocksAddress(conn, header[3]); err != nil {
		t.Fatalf("read bind address: %v", err)
	}
	portReply := make([]byte, 2)
	if _, err := io.ReadFull(conn, portReply); err != nil {
		t.Fatalf("read bind port: %v", err)
	}

	return conn
}

func openClientSocksUDPAssociate(t *testing.T, proxyAddress string) (net.Conn, *net.UDPConn, *net.UDPAddr) {
	t.Helper()

	conn, err := net.Dial("tcp", proxyAddress)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}

	if _, err := conn.Write([]byte{socksVersion, 0x01, socksAuthNone}); err != nil {
		t.Fatalf("write client greeting: %v", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read client greeting reply: %v", err)
	}
	if reply[0] != socksVersion || reply[1] != socksAuthNone {
		t.Fatalf("unexpected greeting reply: %v", reply)
	}

	request := []byte{socksVersion, socksCommandUDPAssociate, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	if _, err := conn.Write(request); err != nil {
		t.Fatalf("write udp associate request: %v", err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read udp associate reply header: %v", err)
	}
	if header[0] != socksVersion || header[1] != socksReplySucceeded {
		t.Fatalf("unexpected udp associate reply header: %v", header)
	}
	_, bindHost, err := readSocksAddress(conn, header[3])
	if err != nil {
		t.Fatalf("read udp associate bind address: %v", err)
	}
	portReply := make([]byte, 2)
	if _, err := io.ReadFull(conn, portReply); err != nil {
		t.Fatalf("read udp associate bind port: %v", err)
	}
	relayPort := int(binary.BigEndian.Uint16(portReply))

	relayAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(bindHost, strconv.Itoa(relayPort)))
	if err != nil {
		t.Fatalf("resolve relay addr: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		t.Fatalf("listen udp client socket: %v", err)
	}

	return conn, udpConn, relayAddr
}

func buildSocksUDPDatagram(t *testing.T, destination string, payload []byte) []byte {
	t.Helper()

	host, portText, err := net.SplitHostPort(destination)
	if err != nil {
		t.Fatalf("split destination: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse destination port: %v", err)
	}
	addressBytes := buildAddressBytes(t, host)
	result := []byte{0x00, 0x00, 0x00}
	result = append(result, addressBytes...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	result = append(result, portBytes...)
	result = append(result, payload...)
	return result
}

func buildAddressBytes(t *testing.T, host string) []byte {
	t.Helper()

	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return append([]byte{0x01}, v4...)
		}
		return append([]byte{0x04}, ip.To16()...)
	}
	hostBytes := []byte(host)
	if len(hostBytes) > 255 {
		t.Fatalf("host too long: %s", host)
	}
	return append([]byte{0x03, byte(len(hostBytes))}, hostBytes...)
}

func waitForAddress(t *testing.T, address string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("address %s did not become ready", address)
}
