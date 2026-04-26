#!/usr/bin/env bash
set -euo pipefail
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup=/root/novpn-backup-$stamp
mkdir -p "$backup"
cp /usr/local/bin/admin-service "$backup/admin-service" 2>/dev/null || true
cp /usr/local/bin/pay-service "$backup/pay-service" 2>/dev/null || true
cp /etc/novpn/pay-service/config.yaml "$backup/pay-service-config.yaml" 2>/dev/null || true
cp /var/lib/novpn/admin/catalog.json "$backup/catalog.json" 2>/dev/null || true
nginx_target=$(readlink -f /etc/nginx/sites-enabled/novpn-admin || printf /etc/nginx/sites-enabled/novpn-admin)
cp "$nginx_target" "$backup/novpn-admin.nginx.conf" 2>/dev/null || true
install -m 0755 /tmp/admin-service-linux-amd64 /usr/local/bin/admin-service
install -m 0755 /tmp/pay-service-linux-amd64 /usr/local/bin/pay-service
install -d /etc/novpn/pay-service /var/lib/novpn/payments /var/lib/novpn/payments/downloads
install -m 0644 /tmp/pay-service-config.yaml /etc/novpn/pay-service/config.yaml
install -m 0644 /tmp/app-debug.apk /var/lib/novpn/payments/downloads/NoVPN-Android.apk
install -m 0644 /tmp/NoVPN-Desktop-Setup-0.1.0.exe /var/lib/novpn/payments/downloads/NoVPN-Desktop-Setup-0.1.0.exe
install -m 0644 /tmp/novpn-admin.nginx.conf "$nginx_target"
python3 - <<'PY'
import json
from datetime import datetime, timezone
from pathlib import Path
path = Path('/var/lib/novpn/admin/catalog.json')
data = json.loads(path.read_text(encoding='utf-8'))
now = datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace('+00:00', 'Z')
plans = data.setdefault('plans', [])
by_id = {item.get('id'): item for item in plans if isinstance(item, dict) and item.get('id')}
plan_defs = [
    {
        'id': 'android-month',
        'name': 'Android / 30 дней',
        'description': 'Код активации для Android и Windows лаунчера',
        'duration_days': 30,
        'traffic_limit_bytes': 0,
        'price_minor': 40,
        'currency': 'RUB',
        'server_ids': ['primary', 'admin-backup'],
        'active': True,
    },
    {
        'id': 'ios-windows-month',
        'name': 'iOS / Windows / 30 дней',
        'description': 'Готовая ссылка для Happ и YAML-профиль для Windows',
        'duration_days': 30,
        'traffic_limit_bytes': 0,
        'price_minor': 60,
        'currency': 'RUB',
        'server_ids': ['primary', 'admin-backup'],
        'active': True,
    },
]
for definition in plan_defs:
    current = by_id.get(definition['id'])
    if current is None:
        current = {'created_at': now}
        plans.append(current)
    current.update(definition)
    current['updated_at'] = now
for item in plans:
    item.setdefault('active', True)
    item.setdefault('created_at', now)
    item['updated_at'] = item.get('updated_at') or now
plans.sort(key=lambda item: item.get('created_at', ''))
data['plans'] = plans
data['version'] = int(data.get('version', 0) or 0) + 1
data['updated_at'] = now
path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
PY
nginx -t
systemctl restart admin-service
systemctl restart pay-service
systemctl reload nginx
systemctl is-active admin-service pay-service nginx
