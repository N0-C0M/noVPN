package acl

import (
	"strings"

	"novpn/pkg/model"
)

type PolicyEvaluator struct {
	allowedNetworks map[string]struct{}
	allowedUpstream map[string]struct{}
}

func NewPolicyEvaluator(allowedNetworks []string, allowedUpstreams []string) PolicyEvaluator {
	networks := make(map[string]struct{}, len(allowedNetworks))
	for _, network := range allowedNetworks {
		normalized := strings.ToLower(strings.TrimSpace(network))
		if normalized == "" {
			continue
		}
		networks[normalized] = struct{}{}
	}

	upstreams := make(map[string]struct{}, len(allowedUpstreams))
	for _, upstream := range allowedUpstreams {
		normalized := strings.TrimSpace(upstream)
		if normalized == "" {
			continue
		}
		upstreams[normalized] = struct{}{}
	}

	return PolicyEvaluator{
		allowedNetworks: networks,
		allowedUpstream: upstreams,
	}
}

func (e PolicyEvaluator) Allow(_ model.Identity, target model.TargetInfo) (model.Decision, error) {
	if len(e.allowedNetworks) > 0 {
		if _, ok := e.allowedNetworks[strings.ToLower(strings.TrimSpace(target.Network))]; !ok {
			return model.Decision{
				Allowed: false,
				Reason:  "network_not_allowed",
			}, nil
		}
	}
	if len(e.allowedUpstream) > 0 {
		if _, ok := e.allowedUpstream[strings.TrimSpace(target.UpstreamAddr)]; !ok {
			return model.Decision{
				Allowed: false,
				Reason:  "upstream_not_allowed",
			}, nil
		}
	}

	return model.Decision{
		Allowed: true,
		Reason:  "policy_allow",
	}, nil
}
