from __future__ import annotations

import tkinter as tk
from pathlib import Path
from tkinter import messagebox

from ..app_catalog_service import AppCatalogService
from ..config_builder import XrayConfigBuilder
from ..models import DesktopSettings
from ..profile_store import ProfileStore
from ..runtime_manager import DesktopRuntimeManager


class MainWindow:
    def __init__(
        self,
        profile_store: ProfileStore,
        builder: XrayConfigBuilder,
        catalog: AppCatalogService,
        output_path: Path,
        runtime_manager: DesktopRuntimeManager,
    ) -> None:
        self._profile_store = profile_store
        self._builder = builder
        self._catalog = catalog
        self._output_path = output_path
        self._runtime_manager = runtime_manager

        self._root = tk.Tk()
        self._root.title("NoVPN Desktop Scaffold")
        self._root.geometry("820x460")

        self._bypass_ru = tk.BooleanVar(value=True)
        self._status = tk.StringVar(value="Config not generated yet")
        self._runtime_status = tk.StringVar(value="Runtime is stopped")

        self._build_layout()

    def run(self) -> int:
        self._root.mainloop()
        return 0

    def _build_layout(self) -> None:
        frame = tk.Frame(self._root, padx=16, pady=16)
        frame.pack(fill=tk.BOTH, expand=True)

        tk.Label(frame, text="NoVPN Client Scaffold", font=("Segoe UI", 16, "bold")).pack(anchor="w")

        tk.Checkbutton(
            frame,
            text="Bypass RU traffic",
            variable=self._bypass_ru,
        ).pack(anchor="w", pady=(12, 8))

        tk.Label(frame, text="Excluded desktop applications").pack(anchor="w")

        self._listbox = tk.Listbox(frame, selectmode=tk.MULTIPLE, height=10)
        self._listbox.pack(fill=tk.BOTH, expand=True, pady=(6, 12))

        for item in self._catalog.list_candidates():
            self._listbox.insert(tk.END, item)

        button_row = tk.Frame(frame)
        button_row.pack(fill=tk.X)

        tk.Button(button_row, text="Generate config.json", command=self._generate_config).pack(side=tk.LEFT)
        tk.Button(button_row, text="Start runtime", command=self._start_runtime).pack(side=tk.LEFT, padx=(12, 0))
        tk.Button(button_row, text="Stop runtime", command=self._stop_runtime).pack(side=tk.LEFT, padx=(12, 0))

        tk.Label(
            button_row,
            text=f"Output file: {self._output_path}",
            anchor="w",
        ).pack(side=tk.LEFT, padx=(12, 0))

        tk.Label(frame, textvariable=self._status, fg="#355070").pack(anchor="w", pady=(12, 0))
        tk.Label(frame, textvariable=self._runtime_status, fg="#355070").pack(anchor="w", pady=(6, 0))

    def _selected_apps(self) -> list[str]:
        return [self._listbox.get(index) for index in self._listbox.curselection()]

    def _current_settings(self) -> DesktopSettings:
        return DesktopSettings(
            bypass_ru=self._bypass_ru.get(),
            excluded_apps=self._selected_apps(),
            output_path=self._output_path,
        )

    def _generate_config(self) -> None:
        profile = self._profile_store.load()
        settings = self._current_settings()
        output_path = self._builder.write(profile, settings)
        self._status.set(f"Generated {output_path}")
        messagebox.showinfo("NoVPN", f"Config generated:\n{output_path}")

    def _start_runtime(self) -> None:
        try:
            profile = self._profile_store.load()
            settings = self._current_settings()
            status = self._runtime_manager.start(profile, settings)
            self._status.set("Runtime configs generated under client/desktop/runtime/generated")
            self._runtime_status.set(status.detail)
            messagebox.showinfo(
                "NoVPN",
                f"Runtime started.\nXray log: {status.xray_log}\nObfuscator log: {status.obfuscator_log}",
            )
        except Exception as exc:
            messagebox.showerror("NoVPN", str(exc))

    def _stop_runtime(self) -> None:
        status = self._runtime_manager.stop()
        self._runtime_status.set(status.detail)
