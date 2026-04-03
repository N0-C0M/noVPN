from __future__ import annotations

import tkinter as tk
from pathlib import Path
from tkinter import messagebox

from ..app_catalog_service import AppCatalogService
from ..config_builder import XrayConfigBuilder
from ..models import DesktopSettings, ProfileOption
from ..profile_store import ProfileStore
from ..runtime_manager import DesktopRuntimeManager


class MainWindow:
    _BG = "#08111F"
    _SURFACE = "#0F1C2E"
    _SURFACE_ALT = "#16263E"
    _ACCENT = "#58E0B5"
    _ACCENT_ALT = "#F78764"
    _TEXT = "#F4F7FB"
    _TEXT_MUTED = "#9CB1CC"
    _CARD = "#132237"
    _CARD_SELECTED = "#1B3553"

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
        self._root.title("NoVPN Desktop")
        self._root.geometry("1040x680")
        self._root.minsize(940, 620)
        self._root.configure(bg=self._BG)

        self._profiles = self._profile_store.available_profiles()
        self._selected_profile_key = tk.StringVar(
            master=self._root,
            value=self._profiles[0].key if self._profiles else self._profile_store.profile_path.name
        )
        self._bypass_ru = tk.BooleanVar(master=self._root, value=True)
        self._status = tk.StringVar(master=self._root, value="Ready to connect")
        self._detail = tk.StringVar(
            master=self._root,
            value="Select a server and press the power button",
        )
        self._selected_apps: set[str] = set()
        self._server_buttons: dict[str, tk.Button] = {}

        self._power_canvas: tk.Canvas | None = None
        self._power_ring: int | None = None
        self._power_core: int | None = None
        self._power_arc: int | None = None
        self._power_line: int | None = None
        self._power_label: int | None = None

        self._build_layout()
        self._sync_preview_config()
        self._refresh_runtime_ui()

    def run(self) -> int:
        self._root.mainloop()
        return 0

    def _build_layout(self) -> None:
        shell = tk.Frame(self._root, bg=self._BG, padx=26, pady=24)
        shell.pack(fill=tk.BOTH, expand=True)
        shell.grid_rowconfigure(1, weight=1)
        shell.grid_columnconfigure(0, weight=1)

        header = tk.Frame(shell, bg=self._BG)
        header.grid(row=0, column=0, sticky="ew")
        header.grid_columnconfigure(0, weight=1)

        tk.Label(
            header,
            text="NoVPN",
            bg=self._BG,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 24),
        ).grid(row=0, column=0, sticky="w")
        tk.Label(
            header,
            text="A compact launcher for VLESS, split routing, and fast server switching.",
            bg=self._BG,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 11),
        ).grid(row=1, column=0, sticky="w", pady=(4, 0))

        tk.Button(
            header,
            text="Settings",
            command=self._open_settings,
            bg=self._SURFACE_ALT,
            fg=self._TEXT,
            activebackground=self._CARD_SELECTED,
            activeforeground=self._TEXT,
            relief=tk.FLAT,
            padx=16,
            pady=8,
            font=("Segoe UI Semibold", 10),
            cursor="hand2",
        ).grid(row=0, column=1, rowspan=2, sticky="ne")

        body = tk.Frame(shell, bg=self._SURFACE, padx=26, pady=24)
        body.grid(row=1, column=0, sticky="nsew", pady=(18, 0))
        body.grid_rowconfigure(1, weight=1)
        body.grid_columnconfigure(0, weight=1)

        hero = tk.Frame(body, bg=self._SURFACE)
        hero.grid(row=0, column=0, sticky="nsew")

        self._power_canvas = tk.Canvas(
            hero,
            width=300,
            height=300,
            bg=self._SURFACE,
            highlightthickness=0,
            bd=0,
            cursor="hand2",
        )
        self._power_canvas.pack(pady=(10, 16))
        self._build_power_button()

        tk.Label(
            hero,
            textvariable=self._status,
            bg=self._SURFACE,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 22),
        ).pack()
        tk.Label(
            hero,
            textvariable=self._detail,
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 11),
            justify="center",
        ).pack(pady=(10, 0))

        footer = tk.Frame(body, bg=self._SURFACE)
        footer.grid(row=1, column=0, sticky="sew", pady=(32, 0))
        footer.grid_columnconfigure(0, weight=1)

        footer_header = tk.Frame(footer, bg=self._SURFACE)
        footer_header.grid(row=0, column=0, sticky="ew")
        footer_header.grid_columnconfigure(0, weight=1)

        tk.Label(
            footer_header,
            text="Available Servers",
            bg=self._SURFACE,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 13),
        ).grid(row=0, column=0, sticky="w")
        tk.Label(
            footer_header,
            text="Settings control RU bypass and app exclusions.",
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 10),
        ).grid(row=0, column=1, sticky="e")

        self._server_row = tk.Frame(footer, bg=self._SURFACE)
        self._server_row.grid(row=1, column=0, sticky="ew", pady=(14, 0))
        self._rebuild_server_cards()

    def _build_power_button(self) -> None:
        assert self._power_canvas is not None
        canvas = self._power_canvas
        self._power_ring = canvas.create_oval(18, 18, 282, 282, fill="#102038", outline="")
        self._power_core = canvas.create_oval(42, 42, 258, 258, fill=self._ACCENT_ALT, outline="")
        self._power_arc = canvas.create_arc(
            106,
            106,
            194,
            194,
            start=38,
            extent=284,
            style=tk.ARC,
            width=10,
            outline=self._TEXT,
        )
        self._power_line = canvas.create_line(
            150,
            86,
            150,
            132,
            width=10,
            fill=self._TEXT,
            capstyle=tk.ROUND,
        )
        self._power_label = canvas.create_text(
            150,
            218,
            text="CONNECT",
            fill=self._TEXT,
            font=("Segoe UI Semibold", 15),
        )
        for item_id in (
            self._power_ring,
            self._power_core,
            self._power_arc,
            self._power_line,
            self._power_label,
        ):
            canvas.tag_bind(item_id, "<Button-1>", lambda _event: self._toggle_runtime())
        canvas.bind("<Button-1>", lambda _event: self._toggle_runtime())

    def _current_settings(self) -> DesktopSettings:
        return DesktopSettings(
            bypass_ru=self._bypass_ru.get(),
            excluded_apps=sorted(self._selected_apps),
            output_path=self._output_path,
        )

    def _current_profile_option(self) -> ProfileOption | None:
        selected_key = self._selected_profile_key.get()
        for option in self._profiles:
            if option.key == selected_key:
                return option
        return self._profiles[0] if self._profiles else None

    def _current_profile(self):
        selected = self._selected_profile_key.get()
        if not selected:
            return self._profile_store.load()
        return self._profile_store.load_by_key(selected)

    def _toggle_runtime(self) -> None:
        if self._runtime_manager.status().running:
            self._stop_runtime()
        else:
            self._start_runtime()

    def _start_runtime(self) -> None:
        try:
            profile = self._current_profile()
            settings = self._current_settings()
            status = self._runtime_manager.start(profile, settings)
            self._status.set("Connected")
            self._detail.set(
                f"{profile.name}\nLogs: {status.xray_log.name} and {status.obfuscator_log.name}"
            )
            self._refresh_runtime_ui()
        except Exception as exc:
            messagebox.showerror("NoVPN", str(exc))

    def _stop_runtime(self) -> None:
        profile = self._current_profile()
        status = self._runtime_manager.stop()
        self._status.set("Paused")
        self._detail.set(f"{profile.name}\n{status.detail}")
        self._refresh_runtime_ui()

    def _refresh_runtime_ui(self) -> None:
        runtime_status = self._runtime_manager.status()
        option = self._current_profile_option()
        if option is not None and not runtime_status.running:
            mode = "RU bypass on" if self._bypass_ru.get() else "Full tunnel mode"
            app_count = len(self._selected_apps)
            self._status.set("Ready" if self._status.get() == "Ready to connect" else self._status.get())
            self._detail.set(
                f"{option.name}  |  {option.address}\n{mode}  |  {app_count} app exclusions"
            )

        if self._power_canvas is None:
            return

        is_running = runtime_status.running
        core_color = self._ACCENT if is_running else self._ACCENT_ALT
        ring_color = "#17304E" if is_running else "#102038"
        label = "DISCONNECT" if is_running else "CONNECT"

        self._power_canvas.itemconfigure(self._power_ring, fill=ring_color)
        self._power_canvas.itemconfigure(self._power_core, fill=core_color)
        self._power_canvas.itemconfigure(self._power_label, text=label)

    def _sync_preview_config(self) -> None:
        try:
            self._builder.write(self._current_profile(), self._current_settings())
        except Exception:
            pass

    def _rebuild_server_cards(self) -> None:
        for child in self._server_row.winfo_children():
            child.destroy()
        self._server_buttons.clear()

        if not self._profiles:
            tk.Label(
                self._server_row,
                text="No profile files found in client/common/profiles/reality.",
                bg=self._SURFACE,
                fg=self._TEXT_MUTED,
                font=("Segoe UI", 10),
            ).pack(anchor="w")
            return

        for option in self._profiles:
            button = tk.Button(
                self._server_row,
                text=f"{option.name}\n{option.address}\nSNI: {option.server_name}",
                command=lambda key=option.key: self._select_profile(key),
                justify="left",
                anchor="w",
                width=26,
                padx=18,
                pady=14,
                relief=tk.FLAT,
                bd=0,
                font=("Segoe UI", 10),
                cursor="hand2",
                wraplength=200,
            )
            button.pack(side=tk.LEFT, padx=(0, 12))
            self._server_buttons[option.key] = button

        self._refresh_server_cards()

    def _refresh_server_cards(self) -> None:
        selected = self._selected_profile_key.get()
        for key, button in self._server_buttons.items():
            if key == selected:
                button.configure(
                    bg=self._CARD_SELECTED,
                    fg=self._TEXT,
                    activebackground=self._CARD_SELECTED,
                    activeforeground=self._TEXT,
                )
            else:
                button.configure(
                    bg=self._CARD,
                    fg=self._TEXT_MUTED,
                    activebackground=self._SURFACE_ALT,
                    activeforeground=self._TEXT,
                )

    def _select_profile(self, profile_key: str) -> None:
        self._selected_profile_key.set(profile_key)
        self._refresh_server_cards()
        self._sync_preview_config()
        runtime_status = self._runtime_manager.status()
        if runtime_status.running:
            self._status.set("Connected")
            self._detail.set("Server changed in UI. Reconnect to apply the new route.")
        else:
            self._status.set("Ready")
        self._refresh_runtime_ui()

    def _open_settings(self) -> None:
        dialog = tk.Toplevel(self._root)
        dialog.title("Settings")
        dialog.geometry("620x520")
        dialog.configure(bg=self._SURFACE)
        dialog.transient(self._root)
        dialog.grab_set()

        panel = tk.Frame(dialog, bg=self._SURFACE, padx=20, pady=20)
        panel.pack(fill=tk.BOTH, expand=True)

        tk.Label(
            panel,
            text="Routing Settings",
            bg=self._SURFACE,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 18),
        ).pack(anchor="w")
        tk.Label(
            panel,
            text="Choose which traffic should stay outside the VPN tunnel.",
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 10),
        ).pack(anchor="w", pady=(4, 16))

        bypass_var = tk.BooleanVar(value=self._bypass_ru.get())
        tk.Checkbutton(
            panel,
            text="Do not proxy RU traffic",
            variable=bypass_var,
            bg=self._SURFACE,
            fg=self._TEXT,
            selectcolor=self._SURFACE_ALT,
            activebackground=self._SURFACE,
            activeforeground=self._TEXT,
            font=("Segoe UI", 10),
        ).pack(anchor="w")

        tk.Label(
            panel,
            text="Excluded desktop applications",
            bg=self._SURFACE,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 11),
        ).pack(anchor="w", pady=(18, 8))

        listbox = tk.Listbox(
            panel,
            selectmode=tk.MULTIPLE,
            height=14,
            bg=self._SURFACE_ALT,
            fg=self._TEXT,
            selectbackground=self._ACCENT,
            selectforeground=self._BG,
            relief=tk.FLAT,
            highlightthickness=0,
            activestyle="none",
        )
        listbox.pack(fill=tk.BOTH, expand=True)

        candidates = self._catalog.list_candidates()
        for item in candidates:
            listbox.insert(tk.END, item)
            if item in self._selected_apps:
                listbox.selection_set(tk.END)

        footer = tk.Frame(panel, bg=self._SURFACE)
        footer.pack(fill=tk.X, pady=(16, 0))

        tk.Label(
            footer,
            text=f"Preview config: {self._output_path}",
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 9),
        ).pack(side=tk.LEFT)

        button_bar = tk.Frame(footer, bg=self._SURFACE)
        button_bar.pack(side=tk.RIGHT)

        tk.Button(
            button_bar,
            text="Cancel",
            command=dialog.destroy,
            bg=self._CARD,
            fg=self._TEXT,
            activebackground=self._SURFACE_ALT,
            activeforeground=self._TEXT,
            relief=tk.FLAT,
            padx=14,
            pady=8,
            cursor="hand2",
        ).pack(side=tk.LEFT)

        tk.Button(
            button_bar,
            text="Save",
            command=lambda: self._save_settings(dialog, bypass_var.get(), candidates, listbox),
            bg=self._ACCENT,
            fg=self._BG,
            activebackground="#7FF0CB",
            activeforeground=self._BG,
            relief=tk.FLAT,
            padx=18,
            pady=8,
            cursor="hand2",
        ).pack(side=tk.LEFT, padx=(10, 0))

    def _save_settings(
        self,
        dialog: tk.Toplevel,
        bypass_ru: bool,
        candidates: list[str],
        listbox: tk.Listbox,
    ) -> None:
        selected = {
            candidates[index]
            for index in listbox.curselection()
            if 0 <= index < len(candidates)
        }
        self._bypass_ru.set(bypass_ru)
        self._selected_apps = selected
        self._sync_preview_config()
        if self._runtime_manager.status().running:
            self._status.set("Connected")
            self._detail.set("Settings saved. Reconnect to apply updated bypass rules.")
        else:
            self._status.set("Ready")
        self._refresh_runtime_ui()
        dialog.destroy()
        messagebox.showinfo("NoVPN", "Settings saved.")
