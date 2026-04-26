from __future__ import annotations

from dataclasses import dataclass, replace
from typing import Callable

from .models import ClientProfile, ConnectionMode, DesktopSettings
from .runtime_preflight import RuntimePreflightChecker, RuntimePreflightReport


@dataclass(slots=True)
class PreparedRuntimeStart:
    settings: DesktopSettings
    preflight: RuntimePreflightReport
    fallback_warning: str = ""


def prepare_runtime_start(
    preflight_checker: RuntimePreflightChecker,
    settings: DesktopSettings,
    *,
    profile_key: str = "",
    profile: ClientProfile | None = None,
    persist_connection_mode: Callable[[ConnectionMode], None] | None = None,
) -> PreparedRuntimeStart:
    report = _evaluate(preflight_checker, profile_key, profile, settings.connection_mode)
    if report.is_ready:
        return PreparedRuntimeStart(settings=settings, preflight=report)

    if report.can_fallback_to_local_proxy(settings.connection_mode):
        effective_settings = replace(settings, connection_mode=ConnectionMode.LOCAL_PROXY)
        fallback_report = _evaluate(
            preflight_checker,
            profile_key,
            profile,
            effective_settings.connection_mode,
        )
        fallback_report.require_ready()
        if persist_connection_mode is not None:
            persist_connection_mode(ConnectionMode.LOCAL_PROXY)
        fallback_warning = report.fallback_warning()
        return PreparedRuntimeStart(
            settings=effective_settings,
            preflight=fallback_report,
            fallback_warning=fallback_warning,
        )

    report.require_ready()
    return PreparedRuntimeStart(settings=settings, preflight=report)


def _evaluate(
    preflight_checker: RuntimePreflightChecker,
    profile_key: str,
    profile: ClientProfile | None,
    connection_mode: ConnectionMode,
) -> RuntimePreflightReport:
    if profile is not None:
        return preflight_checker.evaluate_profile(profile, connection_mode)
    return preflight_checker.evaluate(profile_key, connection_mode)
