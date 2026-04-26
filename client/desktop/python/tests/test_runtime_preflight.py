from __future__ import annotations

import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from novpn_client.models import ConnectionMode
from novpn_client.profile_store import ProfileStore
from novpn_client.runtime_layout import RuntimeLayout
from novpn_client.runtime_preflight import RuntimePreflightChecker

from tests.helpers import build_profile


class RuntimePreflightCheckerTests(unittest.TestCase):
    def test_system_tunnel_without_admin_returns_structured_blocker(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            runtime_root = Path(tmp_dir) / "runtime"
            generated_root = Path(tmp_dir) / "generated"
            bin_dir = runtime_root / "bin"
            bin_dir.mkdir(parents=True, exist_ok=True)
            for name in ("xray.exe", "obfuscator.exe", "geoip.dat", "geosite.dat", "wintun.dll"):
                (bin_dir / name).write_text("stub", encoding="utf-8")

            layout = RuntimeLayout.detect(runtime_root, generated_root)
            store = ProfileStore(
                bundled_profile_path=Path(tmp_dir) / "bundled.profile.json",
                imported_profiles_dir=Path(tmp_dir) / "profiles",
                bootstrap_path=Path(tmp_dir) / "bootstrap.json",
            )
            checker = RuntimePreflightChecker(store, layout)
            checker._windows_tunnel = Mock()
            checker._windows_tunnel.is_windows.return_value = True
            checker._windows_tunnel.is_admin.return_value = False

            report = checker.evaluate_profile(build_profile(), ConnectionMode.SYSTEM_TUNNEL)

        self.assertFalse(report.is_ready)
        self.assertEqual([blocker.code for blocker in report.blockers], ["system_tunnel_admin_required"])
        with self.assertRaises(RuntimeError) as raised:
            report.require_ready()
        self.assertIn("Administrator", str(raised.exception))
        self.assertNotIn("Xray found", str(raised.exception))


if __name__ == "__main__":
    unittest.main()
