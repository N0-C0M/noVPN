from __future__ import annotations

import os
import threading
import tkinter as tk
from dataclasses import replace
from pathlib import Path
from tkinter import filedialog, messagebox

from ..app_catalog_service import AppCatalogService
from ..config_builder import XrayConfigBuilder
from ..device_identity_store import DeviceIdentityStore
from ..invite_redeemer import CodeRedeemKind, CodeRedeemResult, InviteRedeemer
from ..models import (
    AppRoutingMode,
    ClientProfile,
    ClientState,
    DesktopSettings,
    PatternMaskingStrategy,
    ProfileOption,
    TrafficObfuscationStrategy,
)
from ..network_diagnostics import NetworkDiagnosticsRunner
from ..profile_store import ProfileStore
from ..runtime_manager import DesktopRuntimeManager
from ..runtime_preflight import RuntimePreflightChecker
from ..state_store import ClientStateStore

TRAFFIC_LABELS = {
    TrafficObfuscationStrategy.BALANCED: "Сбалансированная",
    TrafficObfuscationStrategy.CDN_MIMIC: "Под CDN",
    TrafficObfuscationStrategy.FRAGMENTED: "Фрагментированная",
    TrafficObfuscationStrategy.MOBILE_MIX: "Мобильная смесь",
    TrafficObfuscationStrategy.TLS_BLEND: "TLS blend",
}

PATTERN_LABELS = {
    PatternMaskingStrategy.STEADY: "Ровный паттерн",
    PatternMaskingStrategy.PULSE: "Пульс",
    PatternMaskingStrategy.RANDOMIZED: "Рандомизированный",
    PatternMaskingStrategy.BURST_FADE: "Всплеск и затухание",
    PatternMaskingStrategy.QUIET_SWEEP: "Тихий sweep",
}


class MainWindow:
    BG = "#02040A"
    SURFACE = "#090E15"
    SURFACE_ALT = "#0E1520"
    ACCENT = "#5FD4A6"
    ACCENT_DIM = "#133627"
    IDLE = "#101722"
    TEXT = "#F3F6FB"
    TEXT_MUTED = "#7E8EA2"
    CARD = "#0D141D"
    CARD_SELECTED = "#122232"
    STROKE = "#182433"
    STROKE_SELECTED = "#2F4D6C"

    def __init__(
        self,
        profile_store: ProfileStore,
        state_store: ClientStateStore,
        device_identity_store: DeviceIdentityStore,
        builder: XrayConfigBuilder,
        catalog: AppCatalogService,
        output_path: Path,
        runtime_manager: DesktopRuntimeManager,
        invite_redeemer: InviteRedeemer,
        diagnostics_runner: NetworkDiagnosticsRunner,
        preflight_checker: RuntimePreflightChecker,
    ) -> None:
        self._profile_store = profile_store
        self._state_store = state_store
        self._device_identity_store = device_identity_store
        self._builder = builder
        self._catalog = catalog
        self._output_path = output_path
        self._runtime_manager = runtime_manager
        self._invite_redeemer = invite_redeemer
        self._diagnostics_runner = diagnostics_runner
        self._preflight_checker = preflight_checker

        self._root = tk.Tk()
        self._root.title("NoVPN Desktop")
        self._root.geometry("1120x860")
        self._root.minsize(980, 760)
        self._root.configure(bg=self.BG)

        self._state = self._state_store.load()
        self._state = replace(self._state, device_id=self._device_identity_store.device_id())
        self._profile_store.set_force_server_ip_mode(self._state.force_server_ip_mode)
        self._profiles: list[ProfileOption] = []
        self._reload_profiles()

        self._status = tk.StringVar(master=self._root, value="Готово")
        self._detail = tk.StringVar(master=self._root, value="Активируйте код или импортируйте профиль.")
        self._header_server = tk.StringVar(master=self._root, value="Нет активированного профиля")
        self._header_meta = tk.StringVar(master=self._root, value="Десктопный клиент с кодами, диагностикой и живыми настройками.")
        self._diagnostics_summary = tk.StringVar(master=self._root, value="Диагностика ещё не запускалась.")

        self._power_canvas: tk.Canvas | None = None
        self._power_ring: int | None = None
        self._power_core: int | None = None
        self._power_arc: int | None = None
        self._power_line: int | None = None
        self._power_label: int | None = None
        self._server_row: tk.Frame | None = None
        self._server_cards: dict[str, tuple[tk.Frame, tk.Label, tk.Label, tk.Label]] = {}
        self._invite_entry: tk.Entry | None = None
        self._activate_button: tk.Button | None = None
        self._disconnect_device_button: tk.Button | None = None
        self._diagnostics_button: tk.Button | None = None
        self._settings_window: tk.Toplevel | None = None
        self._runtime_note = ""
        self._idle_status_title = "Готово"
        self._idle_status_detail = "Активируйте код или импортируйте профиль."
        self._busy = False

        self._build_layout()
        if self._invite_entry is not None:
            self._invite_entry.insert(0, self._state.invite_code)
        self._sync_preview_config()
        self._render()

    def run(self) -> int:
        self._root.mainloop()
        return 0

    def _build_layout(self) -> None:
        shell = self._create_scrollable_surface(self._root, bg=self.BG, padx=26, pady=22)

        self._build_header(shell).pack(fill=tk.X)
        self._build_hero(shell).pack(fill=tk.X, pady=(20, 0))
        self._build_diagnostics(shell).pack(fill=tk.X, pady=(18, 0))
        self._build_code_section(shell).pack(fill=tk.X, pady=(18, 0))
        self._build_servers(shell).pack(fill=tk.BOTH, expand=True, pady=(18, 0))

    def _create_scrollable_surface(
        self,
        parent: tk.Widget,
        bg: str,
        padx: int,
        pady: int,
    ) -> tk.Frame:
        host = tk.Frame(parent, bg=bg)
        host.pack(fill=tk.BOTH, expand=True)

        canvas = tk.Canvas(host, bg=bg, highlightthickness=0, bd=0)
        scrollbar = tk.Scrollbar(host, orient=tk.VERTICAL, command=canvas.yview)
        canvas.configure(yscrollcommand=scrollbar.set)
        canvas.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
        scrollbar.pack(side=tk.RIGHT, fill=tk.Y)

        content = tk.Frame(canvas, bg=bg, padx=padx, pady=pady)
        window_id = canvas.create_window((0, 0), window=content, anchor="nw")
        content.bind("<Configure>", lambda _event: canvas.configure(scrollregion=canvas.bbox("all")))
        canvas.bind("<Configure>", lambda event: canvas.itemconfigure(window_id, width=event.width))
        self._bind_canvas_wheel(canvas, content)
        return content

    def _bind_canvas_wheel(self, canvas: tk.Canvas, *targets: tk.Widget) -> None:
        def on_mouse_wheel(event: tk.Event) -> None:
            delta = int(event.delta / 120) if event.delta else 0
            if delta:
                canvas.yview_scroll(-delta, "units")

        def on_button_up(_event: tk.Event) -> None:
            canvas.yview_scroll(-1, "units")

        def on_button_down(_event: tk.Event) -> None:
            canvas.yview_scroll(1, "units")

        bind_targets = (canvas, *targets)
        for target in bind_targets:
            target.bind("<Enter>", lambda _event: canvas.bind_all("<MouseWheel>", on_mouse_wheel))
            target.bind("<Leave>", lambda _event: canvas.unbind_all("<MouseWheel>"))
        canvas.bind("<Destroy>", lambda _event: canvas.unbind_all("<MouseWheel>"))
        canvas.bind("<Button-4>", on_button_up)
        canvas.bind("<Button-5>", on_button_down)

    def _build_header(self, parent: tk.Widget) -> tk.Frame:
        row = tk.Frame(parent, bg=self.BG)
        row.grid_columnconfigure(0, weight=1)

        title_block = tk.Frame(row, bg=self.BG)
        title_block.grid(row=0, column=0, sticky="w")
        tk.Label(title_block, text="NoVPN", bg=self.BG, fg=self.TEXT, font=("Segoe UI Semibold", 26)).pack(anchor="w")
        tk.Label(title_block, textvariable=self._header_server, bg=self.BG, fg=self.TEXT, font=("Segoe UI Semibold", 16)).pack(anchor="w", pady=(8, 0))
        tk.Label(title_block, textvariable=self._header_meta, bg=self.BG, fg=self.TEXT_MUTED, font=("Segoe UI", 11)).pack(anchor="w", pady=(4, 0))

        actions = tk.Frame(row, bg=self.BG)
        actions.grid(row=0, column=1, sticky="e")
        self._action_button(actions, "Импорт", self._import_profile).pack(side=tk.LEFT)
        self._action_button(actions, "Настройки", self._open_settings).pack(side=tk.LEFT, padx=(10, 0))
        return row

    def _build_hero(self, parent: tk.Widget) -> tk.Frame:
        frame = self._section(parent)
        tk.Label(frame, text="Нажмите, чтобы включить или выключить подключение", bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 11)).pack(pady=(0, 10))
        self._power_canvas = tk.Canvas(frame, width=300, height=300, bg=self.SURFACE, highlightthickness=0, bd=0, cursor="hand2")
        self._power_canvas.pack()
        self._build_power_button()
        tk.Label(frame, textvariable=self._status, bg=self.SURFACE, fg=self.TEXT, font=("Segoe UI Semibold", 24)).pack(pady=(10, 0))
        tk.Label(frame, textvariable=self._detail, bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 11), justify="center").pack(pady=(12, 0))
        return frame

    def _build_diagnostics(self, parent: tk.Widget) -> tk.Frame:
        frame = self._section(parent)
        tk.Label(frame, text="Диагностика", bg=self.SURFACE, fg=self.TEXT, font=("Segoe UI Semibold", 15)).pack(anchor="w")
        tk.Label(frame, text="Проверяет пинг, скачивание и загрузку через локальный runtime.", bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 10)).pack(anchor="w", pady=(6, 12))
        self._diagnostics_button = tk.Button(
            frame,
            text="Запустить диагностику",
            command=self._run_diagnostics,
            bg=self.SURFACE_ALT,
            fg=self.TEXT,
            activebackground=self.CARD_SELECTED,
            activeforeground=self.TEXT,
            relief=tk.FLAT,
            padx=18,
            pady=10,
            cursor="hand2",
        )
        self._diagnostics_button.pack(anchor="w")
        tk.Label(frame, textvariable=self._diagnostics_summary, bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 10), justify="left").pack(anchor="w", pady=(12, 0))
        return frame

    def _build_code_section(self, parent: tk.Widget) -> tk.Frame:
        frame = self._section(parent)
        tk.Label(frame, text="Ключ или промокод", bg=self.SURFACE, fg=self.TEXT, font=("Segoe UI Semibold", 15)).pack(anchor="w")
        tk.Label(frame, text="Поле общее: сюда вводится и ключ доступа, и промокод на трафик.", bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 10)).pack(anchor="w", pady=(6, 12))
        self._invite_entry = tk.Entry(frame, bg=self.SURFACE_ALT, fg=self.TEXT, insertbackground=self.TEXT, relief=tk.FLAT, font=("Segoe UI", 13))
        self._invite_entry.pack(fill=tk.X, ipady=10)
        self._invite_entry.bind("<KeyRelease>", self._on_invite_changed)

        row = tk.Frame(frame, bg=self.SURFACE)
        row.pack(fill=tk.X, pady=(12, 0))
        row.grid_columnconfigure(0, weight=1)
        row.grid_columnconfigure(1, weight=1)
        self._activate_button = tk.Button(row, text="Активировать", command=self._activate_code, bg=self.SURFACE_ALT, fg=self.TEXT, activebackground=self.CARD_SELECTED, activeforeground=self.TEXT, relief=tk.FLAT, padx=16, pady=10, cursor="hand2")
        self._activate_button.grid(row=0, column=0, sticky="ew", padx=(0, 6))
        self._disconnect_device_button = tk.Button(row, text="Отключить устройство", command=self._disconnect_device, bg=self.SURFACE_ALT, fg=self.TEXT, activebackground=self.CARD_SELECTED, activeforeground=self.TEXT, relief=tk.FLAT, padx=16, pady=10, cursor="hand2")
        self._disconnect_device_button.grid(row=0, column=1, sticky="ew", padx=(6, 0))
        return frame

    def _build_servers(self, parent: tk.Widget) -> tk.Frame:
        frame = self._section(parent)
        tk.Label(frame, text="Серверы", bg=self.SURFACE, fg=self.TEXT, font=("Segoe UI Semibold", 15)).pack(anchor="w")
        tk.Label(frame, text="Импортированные профили и профили, полученные по коду.", bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 10)).pack(anchor="w", pady=(6, 14))
        self._server_row = tk.Frame(frame, bg=self.SURFACE)
        self._server_row.pack(fill=tk.BOTH, expand=True)
        return frame

    def _section(self, parent: tk.Widget) -> tk.Frame:
        return tk.Frame(parent, bg=self.SURFACE, padx=22, pady=20, highlightthickness=1, highlightbackground=self.STROKE)

    def _action_button(self, parent: tk.Widget, text: str, command) -> tk.Button:
        return tk.Button(parent, text=text, command=command, bg=self.SURFACE_ALT, fg=self.TEXT, activebackground=self.CARD_SELECTED, activeforeground=self.TEXT, relief=tk.FLAT, padx=16, pady=10, cursor="hand2")

    def _build_power_button(self) -> None:
        assert self._power_canvas is not None
        canvas = self._power_canvas
        self._power_ring = canvas.create_oval(18, 18, 282, 282, fill="#0D141D", outline=self.STROKE, width=3)
        self._power_core = canvas.create_oval(44, 44, 256, 256, fill=self.IDLE, outline="#253344", width=3)
        self._power_arc = canvas.create_arc(104, 104, 196, 196, start=38, extent=284, style=tk.ARC, width=10, outline=self.TEXT)
        self._power_line = canvas.create_line(150, 86, 150, 136, width=10, fill=self.TEXT, capstyle=tk.ROUND)
        self._power_label = canvas.create_text(150, 222, text="ПОДКЛЮЧИТЬ", fill=self.TEXT, font=("Segoe UI Semibold", 14))
        for item_id in (self._power_ring, self._power_core, self._power_arc, self._power_line, self._power_label):
            canvas.tag_bind(item_id, "<Button-1>", lambda _event: self._toggle_runtime())
        canvas.bind("<Button-1>", lambda _event: self._toggle_runtime())

    def _reload_profiles(self, preferred_key: str = "") -> None:
        self._profiles = self._profile_store.available_profiles()
        available_keys = {item.key for item in self._profiles}
        selected_key = preferred_key or self._state.selected_profile_key
        if selected_key not in available_keys:
            selected_key = self._profiles[0].key if self._profiles else ""
        if selected_key != self._state.selected_profile_key:
            self._save_state(selected_profile_key=selected_key)

    def _save_state(self, **changes: object) -> None:
        updated = replace(self._state, **changes)
        updated = replace(updated, selected_apps=self._dedupe(updated.selected_apps))
        updated = replace(updated, invite_code=updated.invite_code.strip(), device_id=self._device_identity_store.device_id())
        self._state = self._state_store.save(updated)
        self._profile_store.set_force_server_ip_mode(self._state.force_server_ip_mode)

    def _dedupe(self, values: list[str]) -> list[str]:
        result: list[str] = []
        for item in values:
            candidate = str(item).strip()
            if candidate and candidate not in result:
                result.append(candidate)
        return result

    def _current_settings(self) -> DesktopSettings:
        return DesktopSettings(
            bypass_ru=self._state.bypass_ru,
            app_routing_mode=self._state.app_routing_mode,
            selected_apps=list(self._state.selected_apps),
            traffic_strategy=self._state.traffic_strategy,
            pattern_strategy=self._state.pattern_strategy,
            device_id=self._state.device_id,
            output_path=self._output_path,
        )

    def _current_profile(self) -> ClientProfile:
        if not self._state.selected_profile_key:
            raise RuntimeError("Сначала активируйте код или импортируйте профиль.")
        return self._profile_store.load_by_key(self._state.selected_profile_key)

    def _current_profile_option(self) -> ProfileOption | None:
        for option in self._profiles:
            if option.key == self._state.selected_profile_key:
                return option
        return self._profiles[0] if self._profiles else None

    def _on_invite_changed(self, _event: tk.Event) -> None:
        if self._invite_entry is None:
            return
        code = self._invite_entry.get().strip()
        if code != self._state.invite_code:
            self._save_state(invite_code=code)

    def _render(self) -> None:
        option = self._current_profile_option()
        runtime_status = self._runtime_manager.status()

        self._header_server.set(option.name if option is not None else "Нет активированного профиля")
        location = option.location_label if option is not None and option.location_label else "не указана"
        self._header_meta.set(f"Локация: {location}. Настройки сохраняются сразу.")

        mode_line = "Режим: обход RU включён" if self._state.bypass_ru else "Режим: полный туннель"
        apps_line = (
            f"Исключено приложений: {len(self._state.selected_apps)}"
            if self._state.app_routing_mode == AppRoutingMode.EXCLUDE_SELECTED
            else f"Только через VPN: {len(self._state.selected_apps)}"
        )
        strategy_line = (
            f"Стратегии: {self._traffic_label(self._state.traffic_strategy)} / "
            f"{self._pattern_label(self._state.pattern_strategy)}"
        )
        base_lines = [
            option.name if option is not None else "Пока нет профилей. Активируйте код или импортируйте YAML/JSON профиль.",
            f"Локация: {location}",
            mode_line,
            apps_line,
            strategy_line,
        ]
        baseline = "\n".join(base_lines)

        if runtime_status.running:
            self._status.set("Подключено")
            runtime_detail = self._runtime_note or (
                f"Логи: {runtime_status.xray_log.name} и {runtime_status.obfuscator_log.name}"
            )
            self._detail.set(runtime_detail + "\n\n" + baseline)
        else:
            self._status.set(self._idle_status_title)
            if self._idle_status_detail:
                self._detail.set(self._idle_status_detail + "\n\n" + baseline)
            else:
                self._detail.set(baseline)

        self._refresh_power(runtime_status.running)
        self._rebuild_server_cards()
        self._refresh_server_cards()
        self._refresh_buttons()

    def _refresh_power(self, is_running: bool) -> None:
        if self._power_canvas is None:
            return
        self._power_canvas.itemconfigure(
            self._power_ring,
            fill="#0F1D18" if is_running else "#0D141D",
            outline=self.STROKE_SELECTED if is_running else self.STROKE,
        )
        self._power_canvas.itemconfigure(
            self._power_core,
            fill=self.ACCENT_DIM if is_running else self.IDLE,
            outline=self.ACCENT if is_running else "#253344",
        )
        self._power_canvas.itemconfigure(
            self._power_label,
            text="ОТКЛЮЧИТЬ" if is_running else "ПОДКЛЮЧИТЬ",
        )

    def _refresh_buttons(self) -> None:
        if self._activate_button is not None:
            self._activate_button.configure(state=tk.DISABLED if self._busy else tk.NORMAL)
        if self._diagnostics_button is not None:
            self._diagnostics_button.configure(
                state=tk.DISABLED if self._busy else tk.NORMAL,
                text="Запустить диагностику" if not self._busy else "Подождите...",
            )
        if self._disconnect_device_button is not None:
            imported = bool(
                self._state.selected_profile_key
                and self._profile_store.is_imported_profile(self._state.selected_profile_key)
            )
            self._disconnect_device_button.configure(
                state=tk.NORMAL if imported and not self._busy else tk.DISABLED
            )

    def _toggle_runtime(self) -> None:
        if self._runtime_manager.status().running:
            self._stop_runtime()
        else:
            self._start_runtime()

    def _start_runtime(self) -> None:
        try:
            self._preflight_checker.evaluate(self._state.selected_profile_key).require_ready()
            profile = self._current_profile()
            status = self._runtime_manager.start(profile, self._current_settings())
            self._runtime_note = f"Логи: {status.xray_log.name} и {status.obfuscator_log.name}"
            self._idle_status_title = "Готово"
            self._idle_status_detail = "Подключение активно."
            self._render()
        except Exception as exc:
            self._idle_status_title = "Нужно исправить окружение"
            self._idle_status_detail = str(exc)
            self._render()
            messagebox.showerror("NoVPN", str(exc))

    def _stop_runtime(self) -> None:
        self._runtime_manager.stop()
        self._runtime_note = ""
        self._idle_status_title = "Остановлено"
        self._idle_status_detail = "Подключение остановлено."
        self._render()

    def _import_profile(self) -> None:
        file_path = filedialog.askopenfilename(
            title="Импорт профиля",
            filetypes=[("Profile files", "*.json *.yaml *.yml"), ("All files", "*.*")],
        )
        if not file_path:
            return
        try:
            option = self._profile_store.import_profile_file(Path(file_path))
            self._reload_profiles(option.key)
            self._idle_status_title = "Готово"
            self._idle_status_detail = f"Профиль импортирован: {option.name}"
            self._sync_preview_config()
            self._render()
            messagebox.showinfo("NoVPN", f"Профиль импортирован: {option.name}")
        except Exception as exc:
            messagebox.showerror("NoVPN", str(exc))

    def _activate_code(self) -> None:
        if self._busy:
            return
        code = self._invite_entry.get().strip() if self._invite_entry is not None else ""
        self._save_state(invite_code=code)
        self._busy = True
        self._idle_status_title = "Активация кода..."
        self._idle_status_detail = "Запрашиваю профиль или бонус трафика у сервера."
        self._render()

        def task() -> tuple[CodeRedeemResult, list[str]]:
            server_address = self._profile_store.bootstrap_server_address()
            if self._state.selected_profile_key:
                server_address = self._current_profile().server.address
            result = self._invite_redeemer.redeem(
                server_address=server_address,
                invite_code=code,
                device_id=self._device_identity_store.device_id(),
                device_name=self._device_identity_store.device_name(),
            )
            profile_keys: list[str] = []
            payloads = [value.strip() for value in result.profile_payloads if value.strip()]
            if not payloads and result.profile_payload.strip():
                payloads = [result.profile_payload.strip()]
            for index, payload in enumerate(payloads):
                option = self._profile_store.import_profile_payload(payload, f"invite-{code}-{index + 1}")
                profile_keys.append(option.key)
            return result, profile_keys

        self._run_async(task, self._on_activate_success, self._on_async_error)

    def _on_activate_success(self, payload: tuple[CodeRedeemResult, list[str]]) -> None:
        result, profile_keys = payload
        if profile_keys:
            if self._runtime_manager.status().running:
                self._runtime_manager.stop()
            self._reload_profiles(profile_keys[0])
            self._runtime_note = ""
            self._sync_preview_config()
            name = result.profile_name or (self._current_profile_option().name if self._current_profile_option() else "профиль")
            self._idle_status_title = "Готово"
            profile_count = len(profile_keys)
            if profile_count > 1:
                self._idle_status_detail = f"Код активирован: {name}. Импортировано профилей: {profile_count}."
            else:
                self._idle_status_detail = f"Код активирован: {name}"
            self._busy = False
            self._render()
            if profile_count > 1:
                messagebox.showinfo("NoVPN", f"Код активирован: {name}\nИмпортировано профилей: {profile_count}")
            else:
                messagebox.showinfo("NoVPN", f"Код активирован: {name}")
            self._start_runtime()
            return

        bonus = self._format_bytes(result.bonus_bytes)
        self._idle_status_title = "Готово"
        self._idle_status_detail = f"Промокод активирован: +{bonus}"
        self._busy = False
        self._render()
        messagebox.showinfo("NoVPN", f"Промокод активирован: +{bonus}")

    def _disconnect_device(self) -> None:
        if self._busy:
            return
        if not self._state.selected_profile_key or not self._profile_store.is_imported_profile(self._state.selected_profile_key):
            messagebox.showinfo("NoVPN", "Текущее устройство не связано с импортированным кодом.")
            return
        profile = self._current_profile()
        profile_key = self._state.selected_profile_key
        self._busy = True
        self._render()

        def task() -> str:
            self._invite_redeemer.disconnect(
                server_address=profile.server.address,
                device_id=self._device_identity_store.device_id(),
                device_name=self._device_identity_store.device_name(),
                client_uuid=profile.server.uuid,
            )
            return profile_key

        self._run_async(task, self._on_disconnect_success, self._on_async_error)

    def _on_disconnect_success(self, profile_key: str) -> None:
        if self._runtime_manager.status().running:
            self._runtime_manager.stop()
        self._profile_store.delete_profile(profile_key)
        self._reload_profiles()
        self._runtime_note = ""
        self._sync_preview_config()
        self._idle_status_title = "Готово"
        self._idle_status_detail = "Устройство отвязано от кода."
        self._busy = False
        self._render()
        messagebox.showinfo("NoVPN", "Устройство отвязано от кода.")

    def _run_diagnostics(self) -> None:
        if self._busy:
            return
        if not self._runtime_manager.status().running:
            messagebox.showerror("NoVPN", "Сначала запустите подключение.")
            return
        self._busy = True
        self._diagnostics_summary.set("Диагностика выполняется...")
        self._render()

        def task() -> str:
            return self._diagnostics_runner.run(self._current_profile()).summary

        self._run_async(task, self._on_diagnostics_success, self._on_async_error)

    def _on_diagnostics_success(self, summary: str) -> None:
        self._busy = False
        self._diagnostics_summary.set(summary)
        self._render()

    def _on_async_error(self, exc: Exception) -> None:
        self._busy = False
        self._idle_status_title = "Ошибка"
        self._idle_status_detail = str(exc)
        self._diagnostics_summary.set(str(exc) if "Latency" not in self._diagnostics_summary.get() else self._diagnostics_summary.get())
        self._render()
        messagebox.showerror("NoVPN", str(exc))

    def _run_async(self, task, on_success, on_error) -> None:
        def worker() -> None:
            try:
                result = task()
            except Exception as exc:
                self._root.after(0, lambda exc=exc: on_error(exc))
            else:
                self._root.after(0, lambda result=result: on_success(result))

        threading.Thread(target=worker, daemon=True).start()

    def _sync_preview_config(self) -> None:
        try:
            self._builder.write(self._current_profile(), self._current_settings())
        except Exception:
            return

    def _select_profile(self, profile_key: str) -> None:
        self._save_state(selected_profile_key=profile_key)
        self._sync_preview_config()
        if self._runtime_manager.status().running:
            self._runtime_note = "Профиль изменён. Переподключитесь, чтобы он начал использоваться."
        else:
            self._idle_status_title = "Готово"
            self._idle_status_detail = "Профиль выбран."
        self._render()

    def _open_settings(self) -> None:
        if self._settings_window is not None and self._settings_window.winfo_exists():
            self._settings_window.focus_force()
            return

        window = tk.Toplevel(self._root)
        window.title("Настройки")
        window.geometry("760x780")
        window.configure(bg=self.SURFACE)
        self._settings_window = window

        panel = self._create_scrollable_surface(window, bg=self.SURFACE, padx=22, pady=22)
        tk.Label(panel, text="Живые настройки", bg=self.SURFACE, fg=self.TEXT, font=("Segoe UI Semibold", 18)).pack(anchor="w")
        tk.Label(panel, text="Все изменения сохраняются сразу. Кнопка сохранения не нужна.", bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 10)).pack(anchor="w", pady=(6, 18))

        bypass_var = tk.BooleanVar(master=window, value=self._state.bypass_ru)
        force_ip_var = tk.BooleanVar(master=window, value=self._state.force_server_ip_mode)
        routing_var = tk.StringVar(master=window, value=self._state.app_routing_mode.value)
        traffic_var = tk.StringVar(master=window, value=self._state.traffic_strategy.value)
        pattern_var = tk.StringVar(master=window, value=self._state.pattern_strategy.value)
        ru_catalog_summary = tk.StringVar(master=window, value="Автоподбор ещё не запускался.")

        tk.Checkbutton(
            panel,
            text="Не проксировать RU-трафик",
            variable=bypass_var,
            command=lambda: self._apply_settings(
                bypass_var.get(),
                force_ip_var.get(),
                AppRoutingMode.from_storage(routing_var.get()),
                TrafficObfuscationStrategy.from_storage(traffic_var.get()),
                PatternMaskingStrategy.from_storage(pattern_var.get()),
            ),
            bg=self.SURFACE,
            fg=self.TEXT,
            selectcolor=self.SURFACE_ALT,
            activebackground=self.SURFACE,
            activeforeground=self.TEXT,
            font=("Segoe UI", 10),
        ).pack(anchor="w")
        tk.Checkbutton(
            panel,
            text="Подменять домен сервера на IP из bootstrap",
            variable=force_ip_var,
            command=lambda: self._apply_settings(
                bypass_var.get(),
                force_ip_var.get(),
                AppRoutingMode.from_storage(routing_var.get()),
                TrafficObfuscationStrategy.from_storage(traffic_var.get()),
                PatternMaskingStrategy.from_storage(pattern_var.get()),
                reload_profiles=True,
            ),
            bg=self.SURFACE,
            fg=self.TEXT,
            selectcolor=self.SURFACE_ALT,
            activebackground=self.SURFACE,
            activeforeground=self.TEXT,
            font=("Segoe UI", 10),
        ).pack(anchor="w", pady=(10, 0))

        self._group_label(panel, "Маршрутизация приложений").pack(anchor="w", pady=(18, 8))
        for mode, text in (
            (AppRoutingMode.EXCLUDE_SELECTED, "Выбранные приложения идут мимо VPN"),
            (AppRoutingMode.ONLY_SELECTED, "Через VPN идут только выбранные приложения"),
        ):
            tk.Radiobutton(
                panel,
                text=text,
                value=mode.value,
                variable=routing_var,
                command=lambda: self._apply_settings(
                    bypass_var.get(),
                    force_ip_var.get(),
                    AppRoutingMode.from_storage(routing_var.get()),
                    TrafficObfuscationStrategy.from_storage(traffic_var.get()),
                    PatternMaskingStrategy.from_storage(pattern_var.get()),
                ),
                bg=self.SURFACE,
                fg=self.TEXT,
                selectcolor=self.SURFACE_ALT,
                activebackground=self.SURFACE,
                activeforeground=self.TEXT,
                font=("Segoe UI", 10),
            ).pack(anchor="w")

        self._group_label(panel, "Маскировка трафика").pack(anchor="w", pady=(18, 8))
        for strategy in TrafficObfuscationStrategy:
            tk.Radiobutton(
                panel,
                text=self._traffic_label(strategy),
                value=strategy.value,
                variable=traffic_var,
                command=lambda: self._apply_settings(
                    bypass_var.get(),
                    force_ip_var.get(),
                    AppRoutingMode.from_storage(routing_var.get()),
                    TrafficObfuscationStrategy.from_storage(traffic_var.get()),
                    PatternMaskingStrategy.from_storage(pattern_var.get()),
                ),
                bg=self.SURFACE,
                fg=self.TEXT,
                selectcolor=self.SURFACE_ALT,
                activebackground=self.SURFACE,
                activeforeground=self.TEXT,
                font=("Segoe UI", 10),
            ).pack(anchor="w")

        self._group_label(panel, "Маскировка паттерна").pack(anchor="w", pady=(18, 8))
        for strategy in PatternMaskingStrategy:
            tk.Radiobutton(
                panel,
                text=self._pattern_label(strategy),
                value=strategy.value,
                variable=pattern_var,
                command=lambda: self._apply_settings(
                    bypass_var.get(),
                    force_ip_var.get(),
                    AppRoutingMode.from_storage(routing_var.get()),
                    TrafficObfuscationStrategy.from_storage(traffic_var.get()),
                    PatternMaskingStrategy.from_storage(pattern_var.get()),
                ),
                bg=self.SURFACE,
                fg=self.TEXT,
                selectcolor=self.SURFACE_ALT,
                activebackground=self.SURFACE,
                activeforeground=self.TEXT,
                font=("Segoe UI", 10),
            ).pack(anchor="w")

        self._group_label(panel, "Каталог RU-приложений").pack(anchor="w", pady=(18, 8))
        tk.Label(
            panel,
            text="Ищет установленные Windows-приложения и отмечает кандидатов как в Android-клиенте.",
            bg=self.SURFACE,
            fg=self.TEXT_MUTED,
            font=("Segoe UI", 10),
        ).pack(anchor="w")

        ru_tools = tk.Frame(panel, bg=self.SURFACE)
        ru_tools.pack(fill=tk.X, pady=(10, 0))
        self._action_button(
            ru_tools,
            "Сканировать и отметить",
            lambda: self._apply_ru_catalog_selection(
                app_list,
                bypass_var,
                force_ip_var,
                routing_var,
                traffic_var,
                pattern_var,
                ru_catalog_summary,
            ),
        ).pack(side=tk.LEFT)
        tk.Label(
            panel,
            textvariable=ru_catalog_summary,
            bg=self.SURFACE,
            fg=self.TEXT_MUTED,
            font=("Segoe UI", 10),
            justify="left",
        ).pack(anchor="w", pady=(10, 0))

        self._group_label(panel, "Приложения Windows").pack(anchor="w", pady=(18, 8))
        app_list = tk.Listbox(
            panel,
            selectmode=tk.MULTIPLE,
            height=10,
            bg=self.SURFACE_ALT,
            fg=self.TEXT,
            selectbackground="#21466A",
            selectforeground=self.TEXT,
            relief=tk.FLAT,
            activestyle="none",
            highlightthickness=0,
        )
        app_list.pack(fill=tk.BOTH, expand=True)
        self._fill_app_list(app_list)
        app_list.bind(
            "<<ListboxSelect>>",
            lambda _event: self._apply_settings(
                bypass_var.get(),
                force_ip_var.get(),
                AppRoutingMode.from_storage(routing_var.get()),
                TrafficObfuscationStrategy.from_storage(traffic_var.get()),
                PatternMaskingStrategy.from_storage(pattern_var.get()),
                selected_apps=self._selected_apps_from_listbox(app_list),
            ),
        )

        footer = tk.Frame(panel, bg=self.SURFACE)
        footer.pack(fill=tk.X, pady=(14, 0))
        tk.Label(footer, text=f"Предпросмотр конфига: {self._output_path}", bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 9)).pack(side=tk.LEFT)

        buttons = tk.Frame(footer, bg=self.SURFACE)
        buttons.pack(side=tk.RIGHT)
        self._action_button(
            buttons,
            "Добавить EXE",
            lambda: self._add_app(
                app_list,
                bypass_var,
                force_ip_var,
                routing_var,
                traffic_var,
                pattern_var,
            ),
        ).pack(side=tk.LEFT)
        self._action_button(
            buttons,
            "Очистить выбор",
            lambda: self._clear_apps(
                app_list,
                bypass_var,
                force_ip_var,
                routing_var,
                traffic_var,
                pattern_var,
            ),
        ).pack(side=tk.LEFT, padx=(8, 0))
        self._action_button(buttons, "Закрыть", window.destroy).pack(side=tk.LEFT, padx=(8, 0))
        window.protocol("WM_DELETE_WINDOW", window.destroy)

    def _apply_settings(
        self,
        bypass_ru: bool,
        force_server_ip_mode: bool,
        app_routing_mode: AppRoutingMode,
        traffic_strategy: TrafficObfuscationStrategy,
        pattern_strategy: PatternMaskingStrategy,
        selected_apps: list[str] | None = None,
        reload_profiles: bool = False,
    ) -> None:
        self._save_state(
            bypass_ru=bypass_ru,
            force_server_ip_mode=force_server_ip_mode,
            app_routing_mode=app_routing_mode,
            traffic_strategy=traffic_strategy,
            pattern_strategy=pattern_strategy,
            selected_apps=list(selected_apps if selected_apps is not None else self._state.selected_apps),
        )
        if reload_profiles:
            self._reload_profiles()
        self._sync_preview_config()
        if self._runtime_manager.status().running:
            self._runtime_note = "Настройки изменены. Переподключитесь, чтобы применить их к текущей сессии."
        else:
            self._idle_status_title = "Готово"
            self._idle_status_detail = "Настройки уже сохранены и применяются сразу."
        self._render()

    def _fill_app_list(self, listbox: tk.Listbox) -> None:
        listbox.delete(0, tk.END)
        candidates = self._catalog.list_candidates(self._state.selected_apps)
        for index, item in enumerate(candidates):
            listbox.insert(tk.END, item)
            if item in self._state.selected_apps:
                listbox.selection_set(index)

    def _selected_apps_from_listbox(self, listbox: tk.Listbox) -> list[str]:
        values = listbox.get(0, tk.END)
        return [values[index] for index in listbox.curselection() if 0 <= index < len(values)]

    def _add_app(
        self,
        listbox: tk.Listbox,
        bypass_var: tk.BooleanVar,
        force_ip_var: tk.BooleanVar,
        routing_var: tk.StringVar,
        traffic_var: tk.StringVar,
        pattern_var: tk.StringVar,
    ) -> None:
        file_path = filedialog.askopenfilename(title="Добавить приложение", filetypes=[("Windows executable", "*.exe"), ("All files", "*.*")])
        if not file_path:
            return
        normalized = self._catalog.normalize_executable(file_path)
        if not normalized:
            messagebox.showerror("NoVPN", "Выберите существующий .exe файл.")
            return
        selected = list(self._state.selected_apps)
        if normalized not in selected:
            selected.append(normalized)
        self._apply_settings(
            bypass_var.get(),
            force_ip_var.get(),
            AppRoutingMode.from_storage(routing_var.get()),
            TrafficObfuscationStrategy.from_storage(traffic_var.get()),
            PatternMaskingStrategy.from_storage(pattern_var.get()),
            selected_apps=selected,
        )
        self._fill_app_list(listbox)

    def _apply_ru_catalog_selection(
        self,
        listbox: tk.Listbox,
        bypass_var: tk.BooleanVar,
        force_ip_var: tk.BooleanVar,
        routing_var: tk.StringVar,
        traffic_var: tk.StringVar,
        pattern_var: tk.StringVar,
        summary_var: tk.StringVar,
    ) -> None:
        suggested_apps, matched_labels = self._catalog.suggest_ru_candidates(self._state.selected_apps)
        if not suggested_apps:
            summary_var.set("Совпадений не найдено. Добавьте приложение вручную через кнопку «Добавить EXE».")
            return

        current = list(self._state.selected_apps)
        selected = self._dedupe([*current, *suggested_apps])
        added_count = len([item for item in selected if item not in current])
        self._apply_settings(
            bypass_var.get(),
            force_ip_var.get(),
            AppRoutingMode.from_storage(routing_var.get()),
            TrafficObfuscationStrategy.from_storage(traffic_var.get()),
            PatternMaskingStrategy.from_storage(pattern_var.get()),
            selected_apps=selected,
        )
        self._fill_app_list(listbox)

        preview_labels = ", ".join(matched_labels[:3])
        if preview_labels:
            summary_var.set(f"Добавлено {added_count} приложений. Найдены: {preview_labels}.")
        else:
            summary_var.set(f"Добавлено {added_count} приложений из локального каталога.")

    def _clear_apps(
        self,
        listbox: tk.Listbox,
        bypass_var: tk.BooleanVar,
        force_ip_var: tk.BooleanVar,
        routing_var: tk.StringVar,
        traffic_var: tk.StringVar,
        pattern_var: tk.StringVar,
    ) -> None:
        self._apply_settings(
            bypass_var.get(),
            force_ip_var.get(),
            AppRoutingMode.from_storage(routing_var.get()),
            TrafficObfuscationStrategy.from_storage(traffic_var.get()),
            PatternMaskingStrategy.from_storage(pattern_var.get()),
            selected_apps=[],
        )
        self._fill_app_list(listbox)

    def _rebuild_server_cards(self) -> None:
        if self._server_row is None:
            return
        for child in self._server_row.winfo_children():
            child.destroy()
        self._server_cards.clear()

        if not self._profiles:
            tk.Label(self._server_row, text="Пока нет профилей. Активируйте код или импортируйте YAML/JSON профиль.", bg=self.SURFACE, fg=self.TEXT_MUTED, font=("Segoe UI", 11), justify="left").pack(anchor="w")
            return

        for option in self._profiles:
            card = tk.Frame(self._server_row, bg=self.CARD, padx=16, pady=16, highlightthickness=1, highlightbackground=self.STROKE, cursor="hand2")
            title = tk.Label(card, text=option.name, bg=self.CARD, fg=self.TEXT, font=("Segoe UI Semibold", 13))
            location = tk.Label(card, text=option.location_label or "не указана", bg=self.CARD, fg=self.TEXT_MUTED, font=("Segoe UI", 10))
            source = tk.Label(card, text="Импортирован", bg=self.CARD, fg="#7ACAA7", font=("Segoe UI", 10))
            title.pack(anchor="w")
            location.pack(anchor="w", pady=(8, 0))
            source.pack(anchor="w", pady=(8, 0))
            card.pack(fill=tk.X, pady=(0, 10))
            for widget in (card, title, location, source):
                widget.bind("<Button-1>", lambda _event, key=option.key: self._select_profile(key))
            self._server_cards[option.key] = (card, title, location, source)

    def _refresh_server_cards(self) -> None:
        for key, (card, title, location, source) in self._server_cards.items():
            selected = key == self._state.selected_profile_key
            fill = self.CARD_SELECTED if selected else self.CARD
            stroke = self.STROKE_SELECTED if selected else self.STROKE
            card.configure(bg=fill, highlightbackground=stroke)
            title.configure(bg=fill)
            location.configure(bg=fill, fg="#D5E3F1" if selected else self.TEXT_MUTED)
            source.configure(bg=fill, fg="#8CE6B9" if selected else "#7ACAA7")

    def _group_label(self, parent: tk.Widget, text: str) -> tk.Label:
        return tk.Label(parent, text=text, bg=self.SURFACE, fg=self.TEXT, font=("Segoe UI Semibold", 12))

    def _traffic_label(self, strategy: TrafficObfuscationStrategy) -> str:
        return TRAFFIC_LABELS[strategy]

    def _pattern_label(self, strategy: PatternMaskingStrategy) -> str:
        return PATTERN_LABELS[strategy]

    def _format_bytes(self, value: int) -> str:
        if value <= 0:
            return "0 B"
        units = ["B", "KiB", "MiB", "GiB", "TiB"]
        current = float(value)
        unit_index = 0
        while current >= 1024.0 and unit_index < len(units) - 1:
            current /= 1024.0
            unit_index += 1
        return f"{current:.1f} {units[unit_index]}"
