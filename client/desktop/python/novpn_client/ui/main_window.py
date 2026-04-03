from __future__ import annotations

import os
import tkinter as tk
from pathlib import Path
from tkinter import messagebox

from ..app_catalog_service import AppCatalogService
from ..config_builder import XrayConfigBuilder
from ..models import ClientProfile, DesktopSettings, ProfileOption
from ..profile_store import ProfileStore
from ..runtime_manager import DesktopRuntimeManager

_TEXTS = {
    "ru": {
        "window_title": "NoVPN Desktop",
        "title": "NoVPN",
        "subtitle": "Тёмный клиент для управления туннелем и быстрым выбором серверов.",
        "settings": "Настройки",
        "power_hint": "Нажмите, чтобы включить или выключить туннель",
        "ready_to_connect": "Готово к подключению",
        "select_server": "Выберите сервер и нажмите центральную кнопку",
        "servers": "Доступные серверы",
        "servers_note": "Маршрутизация RU и исключения приложений находятся в настройках.",
        "connect": "ВКЛЮЧИТЬ",
        "disconnect": "ВЫКЛЮЧИТЬ",
        "connected": "Подключено",
        "paused": "Остановлено",
        "ready": "Готово",
        "ru_bypass_on": "Обход RU-трафика включён",
        "full_tunnel": "Полный туннель",
        "app_exclusions": "{count} исключений приложений",
        "sni": "SNI: {server_name}",
        "logs": "Логи: {xray_log} и {obfuscator_log}",
        "runtime_generated": "Конфиги runtime сохранены в client/desktop/runtime/generated",
        "no_profiles": "В client/common/profiles/reality не найдено ни одного *.profile.json.",
        "server_changed": "Сервер изменён. Переподключитесь, чтобы применить новый маршрут.",
        "settings_title": "Настройки маршрутизации",
        "settings_subtitle": "Выберите, какой трафик должен идти в обход VPN-туннеля.",
        "bypass_ru": "Не проксировать RU-трафик",
        "excluded_apps": "Исключённые приложения Windows",
        "preview": "Предпросмотр конфига: {path}",
        "cancel": "Отмена",
        "save": "Сохранить",
        "settings_saved": "Настройки сохранены.",
        "error_title": "NoVPN",
        "info_title": "NoVPN",
    },
    "en": {
        "window_title": "NoVPN Desktop",
        "title": "NoVPN",
        "subtitle": "Dark tunnel control client with fast server switching.",
        "settings": "Settings",
        "power_hint": "Press to toggle the tunnel",
        "ready_to_connect": "Ready to connect",
        "select_server": "Pick a server and press the center button",
        "servers": "Available servers",
        "servers_note": "RU routing and app exclusions live in settings.",
        "connect": "CONNECT",
        "disconnect": "DISCONNECT",
        "connected": "Connected",
        "paused": "Stopped",
        "ready": "Ready",
        "ru_bypass_on": "RU bypass enabled",
        "full_tunnel": "Full tunnel",
        "app_exclusions": "{count} app exclusions",
        "sni": "SNI: {server_name}",
        "logs": "Logs: {xray_log} and {obfuscator_log}",
        "runtime_generated": "Runtime configs saved in client/desktop/runtime/generated",
        "no_profiles": "No *.profile.json files found in client/common/profiles/reality.",
        "server_changed": "Server changed. Reconnect to apply the new route.",
        "settings_title": "Routing settings",
        "settings_subtitle": "Choose which traffic should bypass the VPN tunnel.",
        "bypass_ru": "Do not proxy RU traffic",
        "excluded_apps": "Excluded Windows applications",
        "preview": "Config preview: {path}",
        "cancel": "Cancel",
        "save": "Save",
        "settings_saved": "Settings saved.",
        "error_title": "NoVPN",
        "info_title": "NoVPN",
    },
}


class MainWindow:
    _BG = "#02040A"
    _SURFACE = "#090E15"
    _SURFACE_ALT = "#0E1520"
    _ACCENT = "#5FD4A6"
    _ACCENT_DIM = "#133627"
    _IDLE = "#101722"
    _TEXT = "#F3F6FB"
    _TEXT_MUTED = "#7E8EA2"
    _CARD = "#0D141D"
    _CARD_SELECTED = "#122232"
    _STROKE = "#182433"
    _STROKE_SELECTED = "#2F4D6C"

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
        self._lang = "en" if os.environ.get("NOVPN_LANG", "ru").lower().startswith("en") else "ru"

        self._root = tk.Tk()
        self._root.title(self._t("window_title"))
        self._root.geometry("1060x700")
        self._root.minsize(960, 640)
        self._root.configure(bg=self._BG)

        self._profiles = self._profile_store.available_profiles()
        self._selected_profile_key = tk.StringVar(
            master=self._root,
            value=self._profiles[0].key if self._profiles else self._profile_store.profile_path.name,
        )
        self._bypass_ru = tk.BooleanVar(master=self._root, value=True)
        self._status = tk.StringVar(master=self._root, value=self._t("ready_to_connect"))
        self._detail = tk.StringVar(master=self._root, value=self._t("select_server"))
        self._selected_apps: set[str] = set()
        self._server_cards: dict[str, dict[str, object]] = {}

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

    def _t(self, key: str, **kwargs: object) -> str:
        text = _TEXTS[self._lang].get(key, _TEXTS["ru"][key])
        return text.format(**kwargs)

    def _build_layout(self) -> None:
        shell = tk.Frame(self._root, bg=self._BG, padx=28, pady=24)
        shell.pack(fill=tk.BOTH, expand=True)
        shell.grid_rowconfigure(1, weight=1)
        shell.grid_columnconfigure(0, weight=1)

        header = tk.Frame(shell, bg=self._BG)
        header.grid(row=0, column=0, sticky="ew")
        header.grid_columnconfigure(0, weight=1)

        title_block = tk.Frame(header, bg=self._BG)
        title_block.grid(row=0, column=0, sticky="w")

        tk.Label(
            title_block,
            text=self._t("title"),
            bg=self._BG,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 24),
        ).pack(anchor="w")
        tk.Label(
            title_block,
            text=self._t("subtitle"),
            bg=self._BG,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 11),
        ).pack(anchor="w", pady=(4, 0))

        self._build_chip_button(
            header,
            text=self._t("settings"),
            command=self._open_settings,
            width=126,
            height=42,
            fill=self._SURFACE_ALT,
            border=self._STROKE,
            text_color=self._TEXT,
        ).grid(row=0, column=1, sticky="ne", pady=(4, 0))

        body = tk.Frame(shell, bg=self._SURFACE, padx=28, pady=26, highlightthickness=1, highlightbackground=self._STROKE)
        body.grid(row=1, column=0, sticky="nsew", pady=(20, 0))
        body.grid_rowconfigure(1, weight=1)
        body.grid_columnconfigure(0, weight=1)

        hero = tk.Frame(body, bg=self._SURFACE)
        hero.grid(row=0, column=0, sticky="nsew")

        tk.Label(
            hero,
            text=self._t("power_hint"),
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 11),
        ).pack(pady=(0, 8))

        self._power_canvas = tk.Canvas(
            hero,
            width=310,
            height=310,
            bg=self._SURFACE,
            highlightthickness=0,
            bd=0,
            cursor="hand2",
        )
        self._power_canvas.pack(pady=(6, 12))
        self._build_power_button()

        tk.Label(
            hero,
            textvariable=self._status,
            bg=self._SURFACE,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 24),
        ).pack()
        tk.Label(
            hero,
            textvariable=self._detail,
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 11),
            justify="center",
        ).pack(pady=(12, 0))

        footer = tk.Frame(body, bg=self._SURFACE)
        footer.grid(row=1, column=0, sticky="sew", pady=(34, 0))
        footer.grid_columnconfigure(0, weight=1)

        footer_header = tk.Frame(footer, bg=self._SURFACE)
        footer_header.grid(row=0, column=0, sticky="ew")
        footer_header.grid_columnconfigure(0, weight=1)

        tk.Label(
            footer_header,
            text=self._t("servers"),
            bg=self._SURFACE,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 13),
        ).grid(row=0, column=0, sticky="w")
        tk.Label(
            footer_header,
            text=self._t("servers_note"),
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 10),
        ).grid(row=0, column=1, sticky="e")

        self._server_row = tk.Frame(footer, bg=self._SURFACE)
        self._server_row.grid(row=1, column=0, sticky="ew", pady=(16, 0))
        self._rebuild_server_cards()

    def _build_power_button(self) -> None:
        assert self._power_canvas is not None
        canvas = self._power_canvas
        self._power_ring = canvas.create_oval(
            16,
            16,
            294,
            294,
            fill="#0D141D",
            outline=self._STROKE,
            width=3,
        )
        self._power_core = canvas.create_oval(
            42,
            42,
            268,
            268,
            fill=self._IDLE,
            outline="#253344",
            width=3,
        )
        self._power_arc = canvas.create_arc(
            104,
            104,
            206,
            206,
            start=38,
            extent=284,
            style=tk.ARC,
            width=10,
            outline=self._TEXT,
        )
        self._power_line = canvas.create_line(
            155,
            84,
            155,
            136,
            width=10,
            fill=self._TEXT,
            capstyle=tk.ROUND,
        )
        self._power_label = canvas.create_text(
            155,
            225,
            text=self._t("connect"),
            fill=self._TEXT,
            font=("Segoe UI Semibold", 14),
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

    def _current_profile(self) -> ClientProfile:
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
            self._status.set(self._t("connected"))
            self._detail.set(
                f"{profile.name}\n{self._t('logs', xray_log=status.xray_log.name, obfuscator_log=status.obfuscator_log.name)}"
            )
            self._refresh_runtime_ui()
        except Exception as exc:
            messagebox.showerror(self._t("error_title"), str(exc))

    def _stop_runtime(self) -> None:
        profile = self._current_profile()
        self._runtime_manager.stop()
        self._status.set(self._t("paused"))
        self._detail.set(f"{profile.name}\n{self._t('paused')}")
        self._refresh_runtime_ui()

    def _refresh_runtime_ui(self) -> None:
        runtime_status = self._runtime_manager.status()
        option = self._current_profile_option()
        if option is not None and not runtime_status.running:
            mode = self._t("ru_bypass_on") if self._bypass_ru.get() else self._t("full_tunnel")
            app_count = self._t("app_exclusions", count=len(self._selected_apps))
            if self._status.get() in {self._t("ready_to_connect"), self._t("ready"), self._t("paused")}:
                self._status.set(self._t("ready"))
            self._detail.set(f"{option.name}  |  {option.address}\n{mode}  |  {app_count}")

        if self._power_canvas is None:
            return

        is_running = runtime_status.running
        ring_fill = "#0F1D18" if is_running else "#0D141D"
        core_fill = self._ACCENT_DIM if is_running else self._IDLE
        core_outline = self._ACCENT if is_running else "#253344"
        label = self._t("disconnect") if is_running else self._t("connect")

        self._power_canvas.itemconfigure(self._power_ring, fill=ring_fill, outline=self._STROKE_SELECTED if is_running else self._STROKE)
        self._power_canvas.itemconfigure(self._power_core, fill=core_fill, outline=core_outline)
        self._power_canvas.itemconfigure(self._power_label, text=label)

    def _sync_preview_config(self) -> None:
        try:
            self._builder.write(self._current_profile(), self._current_settings())
        except Exception:
            pass

    def _rebuild_server_cards(self) -> None:
        for child in self._server_row.winfo_children():
            child.destroy()
        self._server_cards.clear()

        if not self._profiles:
            tk.Label(
                self._server_row,
                text=self._t("no_profiles"),
                bg=self._SURFACE,
                fg=self._TEXT_MUTED,
                font=("Segoe UI", 10),
            ).pack(anchor="w")
            return

        for option in self._profiles:
            canvas = tk.Canvas(
                self._server_row,
                width=240,
                height=124,
                bg=self._SURFACE,
                highlightthickness=0,
                bd=0,
                cursor="hand2",
            )
            background_id = self._create_rounded_rect(
                canvas,
                4,
                4,
                236,
                120,
                28,
                fill=self._CARD,
                outline=self._STROKE,
                width=2,
            )
            title_id = canvas.create_text(
                22,
                24,
                anchor="nw",
                text=option.name,
                width=190,
                fill=self._TEXT,
                font=("Segoe UI Semibold", 12),
            )
            address_id = canvas.create_text(
                22,
                62,
                anchor="nw",
                text=option.address,
                width=190,
                fill=self._TEXT_MUTED,
                font=("Segoe UI", 10),
            )
            sni_id = canvas.create_text(
                22,
                88,
                anchor="nw",
                text=self._t("sni", server_name=option.server_name),
                width=190,
                fill="#6E87A2",
                font=("Segoe UI", 10),
            )

            for item_id in (background_id, title_id, address_id, sni_id):
                canvas.tag_bind(item_id, "<Button-1>", lambda _event, key=option.key: self._select_profile(key))
            canvas.bind("<Button-1>", lambda _event, key=option.key: self._select_profile(key))
            canvas.pack(side=tk.LEFT, padx=(0, 14))

            self._server_cards[option.key] = {
                "canvas": canvas,
                "background": background_id,
                "title": title_id,
                "address": address_id,
                "sni": sni_id,
            }

        self._refresh_server_cards()

    def _refresh_server_cards(self) -> None:
        selected = self._selected_profile_key.get()
        for key, card in self._server_cards.items():
            canvas = card["canvas"]
            background = card["background"]
            title = card["title"]
            address = card["address"]
            sni = card["sni"]

            is_selected = key == selected
            fill = self._CARD_SELECTED if is_selected else self._CARD
            outline = self._STROKE_SELECTED if is_selected else self._STROKE
            address_color = "#D5E3F1" if is_selected else self._TEXT_MUTED

            assert isinstance(canvas, tk.Canvas)
            assert isinstance(background, int)
            assert isinstance(title, int)
            assert isinstance(address, int)
            assert isinstance(sni, int)

            canvas.itemconfigure(background, fill=fill, outline=outline)
            canvas.itemconfigure(title, fill=self._TEXT)
            canvas.itemconfigure(address, fill=address_color)
            canvas.itemconfigure(sni, fill="#7D9ABE" if is_selected else "#6E87A2")

    def _select_profile(self, profile_key: str) -> None:
        self._selected_profile_key.set(profile_key)
        self._refresh_server_cards()
        self._sync_preview_config()
        runtime_status = self._runtime_manager.status()
        if runtime_status.running:
            self._status.set(self._t("connected"))
            self._detail.set(self._t("server_changed"))
        else:
            self._status.set(self._t("ready"))
        self._refresh_runtime_ui()

    def _open_settings(self) -> None:
        dialog = tk.Toplevel(self._root)
        dialog.title(self._t("settings"))
        dialog.geometry("640x540")
        dialog.configure(bg=self._SURFACE)
        dialog.transient(self._root)
        dialog.grab_set()

        panel = tk.Frame(
            dialog,
            bg=self._SURFACE,
            padx=22,
            pady=22,
            highlightthickness=1,
            highlightbackground=self._STROKE,
        )
        panel.pack(fill=tk.BOTH, expand=True, padx=12, pady=12)

        tk.Label(
            panel,
            text=self._t("settings_title"),
            bg=self._SURFACE,
            fg=self._TEXT,
            font=("Segoe UI Semibold", 18),
        ).pack(anchor="w")
        tk.Label(
            panel,
            text=self._t("settings_subtitle"),
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 10),
        ).pack(anchor="w", pady=(4, 16))

        bypass_var = tk.BooleanVar(master=dialog, value=self._bypass_ru.get())
        tk.Checkbutton(
            panel,
            text=self._t("bypass_ru"),
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
            text=self._t("excluded_apps"),
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
            selectbackground="#21466A",
            selectforeground=self._TEXT,
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
            text=self._t("preview", path=self._output_path),
            bg=self._SURFACE,
            fg=self._TEXT_MUTED,
            font=("Segoe UI", 9),
        ).pack(side=tk.LEFT)

        button_bar = tk.Frame(footer, bg=self._SURFACE)
        button_bar.pack(side=tk.RIGHT)

        tk.Button(
            button_bar,
            text=self._t("cancel"),
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
            text=self._t("save"),
            command=lambda: self._save_settings(dialog, bypass_var.get(), candidates, listbox),
            bg=self._SURFACE_ALT,
            fg=self._TEXT,
            activebackground=self._CARD_SELECTED,
            activeforeground=self._TEXT,
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
            self._status.set(self._t("connected"))
            self._detail.set(self._t("server_changed"))
        else:
            self._status.set(self._t("ready"))
        self._refresh_runtime_ui()
        dialog.destroy()
        messagebox.showinfo(self._t("info_title"), self._t("settings_saved"))

    def _build_chip_button(
        self,
        parent: tk.Widget,
        text: str,
        command,
        width: int,
        height: int,
        fill: str,
        border: str,
        text_color: str,
    ) -> tk.Canvas:
        canvas = tk.Canvas(
            parent,
            width=width,
            height=height,
            bg=self._BG,
            highlightthickness=0,
            bd=0,
            cursor="hand2",
        )
        background_id = self._create_rounded_rect(
            canvas,
            2,
            2,
            width - 2,
            height - 2,
            20,
            fill=fill,
            outline=border,
            width=2,
        )
        text_id = canvas.create_text(
            width / 2,
            height / 2,
            text=text,
            fill=text_color,
            font=("Segoe UI Semibold", 10),
        )
        for item_id in (background_id, text_id):
            canvas.tag_bind(item_id, "<Button-1>", lambda _event: command())
        canvas.bind("<Button-1>", lambda _event: command())
        return canvas

    def _create_rounded_rect(
        self,
        canvas: tk.Canvas,
        x1: int,
        y1: int,
        x2: int,
        y2: int,
        radius: int,
        **kwargs: object,
    ) -> int:
        points = [
            x1 + radius,
            y1,
            x1 + radius,
            y1,
            x2 - radius,
            y1,
            x2 - radius,
            y1,
            x2,
            y1,
            x2,
            y1 + radius,
            x2,
            y1 + radius,
            x2,
            y2 - radius,
            x2,
            y2 - radius,
            x2,
            y2,
            x2 - radius,
            y2,
            x2 - radius,
            y2,
            x1 + radius,
            y2,
            x1 + radius,
            y2,
            x1,
            y2,
            x1,
            y2 - radius,
            x1,
            y2 - radius,
            x1,
            y1 + radius,
            x1,
            y1 + radius,
            x1,
            y1,
        ]
        return canvas.create_polygon(points, smooth=True, splinesteps=36, **kwargs)
