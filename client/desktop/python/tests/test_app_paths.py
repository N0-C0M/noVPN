from __future__ import annotations

import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from novpn_client.app_paths import _resolve_generated_root


class AppPathsTests(unittest.TestCase):
    def test_repo_run_uses_repo_generated_directory(self) -> None:
        app_root = Path("D:/repo/noVPN")
        with patch.object(sys, "frozen", False, create=True):
            resolved = _resolve_generated_root(app_root)
        self.assertEqual(resolved, app_root / "client" / "desktop" / "python" / "generated")

    def test_frozen_mode_uses_localappdata_generated_directory(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            app_root = Path("D:/portable/NoVPN Desktop")
            with patch.object(sys, "frozen", True, create=True):
                with patch.dict(os.environ, {"LOCALAPPDATA": tmp_dir}, clear=False):
                    resolved = _resolve_generated_root(app_root)
        self.assertEqual(resolved, Path(tmp_dir) / "NoVPN Desktop" / "generated")


if __name__ == "__main__":
    unittest.main()
