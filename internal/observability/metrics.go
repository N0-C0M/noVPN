package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Metrics struct {
	registry *prometheus.Registry

	TCPConnectionsActive     prometheus.Gauge
	TCPAcceptErrorsTotal     prometheus.Counter
	TCPRejectedTotal         prometheus.Counter
	TCPBytesInTotal          prometheus.Counter
	TCPBytesOutTotal         prometheus.Counter
	TCPUpstreamDialFailTotal prometheus.Counter
	UDPPacketsInTotal        prometheus.Counter
	UDPPacketsOutTotal       prometheus.Counter
	UDPPacketsDroppedTotal   prometheus.Counter
	UDPRejectedTotal         prometheus.Counter
	UDPSessionsActive        prometheus.Gauge
	AuthFailuresTotal        prometheus.Counter
	ACLDeniesTotal           prometheus.Counter
	ShutdownInProgress       prometheus.Gauge
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()

	m := &Metrics{
		registry: registry,
		TCPConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_tcp_connections_active",
			Help: "Current number of active TCP client connections.",
		}),
		TCPAcceptErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_tcp_accept_errors_total",
			Help: "Total TCP accept loop errors.",
		}),
		TCPRejectedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_tcp_rejected_total",
			Help: "Total TCP connections rejected by policy or limits.",
		}),
		TCPBytesInTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_tcp_bytes_in_total",
			Help: "Total bytes forwarded from client to upstream.",
		}),
		TCPBytesOutTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_tcp_bytes_out_total",
			Help: "Total bytes forwarded from upstream to client.",
		}),
		TCPUpstreamDialFailTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_tcp_upstream_dial_failures_total",
			Help: "Total TCP upstream dial failures.",
		}),
		UDPPacketsInTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_udp_packets_in_total",
			Help: "Total UDP packets forwarded from client to upstream.",
		}),
		UDPPacketsOutTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_udp_packets_out_total",
			Help: "Total UDP packets forwarded from upstream to client.",
		}),
		UDPPacketsDroppedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_udp_packets_dropped_total",
			Help: "Total dropped UDP packets.",
		}),
		UDPRejectedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_udp_rejected_total",
			Help: "Total UDP packets or sessions rejected by policy or limits.",
		}),
		UDPSessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_udp_sessions_active",
			Help: "Current number of active UDP sessions.",
		}),
		AuthFailuresTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_auth_failures_total",
			Help: "Total authentication failures.",
		}),
		ACLDeniesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_acl_denies_total",
			Help: "Total ACL deny decisions.",
		}),
		ShutdownInProgress: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_shutdown_in_progress",
			Help: "Whether graceful shutdown is currently in progress.",
		}),
	}

	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.TCPConnectionsActive,
		m.TCPAcceptErrorsTotal,
		m.TCPRejectedTotal,
		m.TCPBytesInTotal,
		m.TCPBytesOutTotal,
		m.TCPUpstreamDialFailTotal,
		m.UDPPacketsInTotal,
		m.UDPPacketsOutTotal,
		m.UDPPacketsDroppedTotal,
		m.UDPRejectedTotal,
		m.UDPSessionsActive,
		m.AuthFailuresTotal,
		m.ACLDeniesTotal,
		m.ShutdownInProgress,
	)

	return m
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}
