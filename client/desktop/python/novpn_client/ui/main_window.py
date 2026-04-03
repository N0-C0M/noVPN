from __future__ import annotations

import tkinter as tk
from pathlib import Path
from tkinter import messagebox

from ..app_catalog_service import AppCatalogService
from ..config_builder import XrayConfigBuilder
from ..models import DesktopSettings
from ..profile_store import ProfileStore


class MainWindow:
    def __init__(
        self,
        profile_store: ProfileStore,
        builder: XrayConfigBuilder,
        catalog: AppCatalogService,
        output_path: Path,
    ) -> None:
        self._profile_store = profile_store
        self._builder = builder
        self._catalog = catalog
        self._output_path = output_path

        self._root = tk.Tk()
        self._root.title("NoVPN Desktop Scaffold")
        self._root.geometry("760x420")

        self._bypass_ru = tk.BooleanVar(value=True)
        self._status = tk.StringVar(value="Config not generated yet")

        self._build_layout()

    def run(self) -> int:
        self._root.mainloop()
        return 0

    def _build_layout(self) -> None:
        frame = tk.Frame(self._root, padx=16, pady=16)
        frame.pack(fill=tk.BOTH, expand=True)

        title = tk.Label(frame, text="NoVPN Client Scaffold", font=("Segoe UI", 16, "bold"))
        title.pack(anchor="w")

        tk.Checkbutton(
            frame,
            text="Не проксировать РФ",
            variable=self._bypass_ru,
        ).pack(anchor="w", pady=(12, 8))

        tk.Label(frame, text="Исключённые desktop-приложения").pack(anchor="w")

        self._listbox = tk.Listbox(frame, selectmode=tk.MULTIPLE, height=10)
        self._listbox.pack(fill=tk.BOTH, expand=True, pady=(6, 12))

        for item in self._catalog.list_candidates():
            self._listbox.insert(tk.END, item)

        button_row = tk.Frame(frame)
        button_row.pack(fill=tk.X)

        tk.Button(
            button_row,
            text="Сгенерировать config.json",
            command=self._generate_config,
        ).pack(side=tk.LEFT)

        tk.Label(
            button_row,
            text=f"Выходной файл: {self._output_path}",
            anchor="w",
        ).pack(side=tk.LEFT, padx=(12, 0))

        tk.Label(frame, textvariable=self._status, fg="#355070").pack(anchor="w", pady=(12, 0))

    def _generate_config(self) -> None:
        profile = self._profile_store.load()
        selected = [self._listbox.get(index) for index in self._listbox.curselection()]

        settings = DesktopSettings(
            bypass_ru=self._bypass_ru.get(),
            excluded_apps=selected,
            output_path=self._output_path,
        )
        output_path = self._builder.write(profile, settings)
        self._status.set(f"Generated {output_path}")
        messagebox.showinfo("NoVPN", f"Config generated:\n{output_path}")
