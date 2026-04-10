package reality

import (
	"context"

	"novpn/internal/config"
)

func (p *Provisioner) LoadRegistry() (Registry, error) {
	return p.registryStore.Load()
}

func (p *Provisioner) LoadState() (State, error) {
	return p.loadState()
}

func (p *Provisioner) Config() config.RealityConfig {
	return p.cfg
}

func (p *Provisioner) RegistrySummary() (RegistrySummary, error) {
	return p.registryStore.Summary(p.cfg)
}

func (p *Provisioner) ListClients() ([]ClientRecord, error) {
	return p.registryStore.ListClients()
}

func (p *Provisioner) ListInvites() ([]InviteRecord, error) {
	return p.registryStore.ListInvites()
}

func (p *Provisioner) ListPromos() ([]PromoRecord, error) {
	return p.registryStore.ListPromos()
}

func (p *Provisioner) CreateInvite(input InviteCreateRequest) (InviteRecord, error) {
	return p.registryStore.CreateInvite(input)
}

func (p *Provisioner) CreatePromo(input PromoCreateRequest) (PromoRecord, error) {
	return p.registryStore.CreatePromo(input)
}

func (p *Provisioner) RedeemInvite(ctx context.Context, code string, deviceID string, deviceName string) (InviteRedeemResult, Result, error) {
	redeemResult, err := p.registryStore.RedeemInvite(code, deviceID, deviceName)
	if err != nil {
		return InviteRedeemResult{}, Result{}, err
	}

	refreshResult, err := p.refreshRuntime(ctx)
	if err != nil {
		return InviteRedeemResult{}, Result{}, err
	}

	return redeemResult, refreshResult, nil
}

func (p *Provisioner) RedeemPromo(ctx context.Context, code string, deviceID string, deviceName string) (PromoRedeemResult, Result, error) {
	redeemResult, err := p.registryStore.RedeemPromo(code, deviceID, deviceName)
	if err != nil {
		return PromoRedeemResult{}, Result{}, err
	}

	refreshResult, err := p.refreshRuntime(ctx)
	if err != nil {
		return PromoRedeemResult{}, Result{}, err
	}

	return redeemResult, refreshResult, nil
}

func (p *Provisioner) RevokeClient(ctx context.Context, clientID string) (ClientRecord, Result, error) {
	client, err := p.registryStore.RevokeClient(clientID)
	if err != nil {
		return ClientRecord{}, Result{}, err
	}

	refreshResult, err := p.refreshRuntime(ctx)
	if err != nil {
		return ClientRecord{}, Result{}, err
	}

	return client, refreshResult, nil
}

func (p *Provisioner) DisconnectDevice(ctx context.Context, deviceID string, clientUUID string) (ClientRecord, Result, error) {
	client, err := p.registryStore.DisconnectDevice(deviceID, clientUUID)
	if err != nil {
		return ClientRecord{}, Result{}, err
	}

	refreshResult, err := p.refreshRuntime(ctx)
	if err != nil {
		return ClientRecord{}, Result{}, err
	}

	return client, refreshResult, nil
}

func (p *Provisioner) BuildClientProfileFor(state State, client ClientRecord) ClientProfile {
	return buildClientProfileFor(p.cfg, state, client)
}

func (p *Provisioner) BuildClientProfilesFor(state State, client ClientRecord) []ClientProfile {
	return buildClientProfilesFor(p.cfg, state, client)
}
