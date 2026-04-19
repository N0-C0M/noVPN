from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(slots=True)
class RuntimeLayout:
    root: Path
    generated_root: Path
    xray_binary: Path
    obfuscator_binary: Path
    wintun_dll: Path
    xray_config: Path
    obfuscator_config: Path
    logs_dir: Path
    xray_log: Path
    obfuscator_log: Path

    @classmethod
    def detect(
        cls,
        runtime_root: Path,
        generated_root: Path | None = None,
        xray_binary: Path | None = None,
        obfuscator_binary: Path | None = None,
    ) -> "RuntimeLayout":
        effective_generated_root = generated_root or runtime_root / "generated"
        logs_dir = effective_generated_root / "logs"

        return cls(
            root=runtime_root,
            generated_root=effective_generated_root,
            xray_binary=xray_binary or runtime_root / "bin" / "xray.exe",
            obfuscator_binary=obfuscator_binary or runtime_root / "bin" / "obfuscator.exe",
            wintun_dll=runtime_root / "bin" / "wintun.dll",
            xray_config=effective_generated_root / "xray.config.json",
            obfuscator_config=effective_generated_root / "obfuscator.config.json",
            logs_dir=logs_dir,
            xray_log=logs_dir / "xray.log",
            obfuscator_log=logs_dir / "obfuscator.log",
        )

    def ensure_directories(self) -> None:
        self.logs_dir.mkdir(parents=True, exist_ok=True)
