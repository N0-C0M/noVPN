package reality

import "context"

func (p *Provisioner) ExportTrafficUsages(ctx context.Context) (map[string]TrafficUsage, error) {
	return p.queryTrafficUsages(ctx)
}

func (p *Provisioner) ApplyRemoteRegistry(ctx context.Context, remote Registry) (bool, Result, error) {
	changed, _, err := p.registryStore.MergeRemote(remote)
	if err != nil {
		return false, Result{}, err
	}
	if !changed {
		return false, Result{
			ConfigPath:        p.cfg.Xray.ConfigPath,
			StatePath:         p.cfg.Xray.StatePath,
			RegistryPath:      p.cfg.Xray.RegistryPath,
			ClientProfilePath: p.cfg.Xray.ClientProfilePath,
		}, nil
	}
	result, err := p.Bootstrap(ctx, Options{
		InstallXray:    false,
		ValidateConfig: false,
		ManageService:  true,
	})
	if err != nil {
		return true, Result{}, err
	}
	return true, result, nil
}
