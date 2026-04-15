# Split Deployment

- `2.26.85.47`: `admin-service` and `pay-service`
- `87.121.105.190`: `vpn-service`

## Required state

The admin control plane must have the same Reality state as the VPN node. Before the first
`admin-service` start, copy the active VPN node files to the admin host:

```bash
scp root@87.121.105.190:/var/lib/novpn/reality/state.yaml /var/lib/novpn/reality/state.yaml
scp root@87.121.105.190:/var/lib/novpn/reality/registry.json /var/lib/novpn/reality/registry.json
```

## First bootstrap

On the VPN node run one bootstrap pass before enabling `vpn-service`:

```bash
sudo /usr/local/bin/reality-bootstrap -config /etc/novpn/vpn-service/config.yaml
```

Then enable services:

```bash
sudo systemctl enable --now admin-service
sudo systemctl enable --now pay-service
sudo systemctl enable --now vpn-service
```

## Android bootstrap

The Android bootstrap file should point to the admin host as the control plane and can keep the VPN
profile endpoint separate:

```bash
go run ./cmd/client-profile-sync \
  -input /var/lib/novpn/reality/client-profile.yaml \
  -bootstrap-address 2.26.85.47 \
  -bootstrap-api-base http://2.26.85.47/admin
```
