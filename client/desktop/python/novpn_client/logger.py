from __future__ import annotations

import logging
import os
import sys
import threading
from logging.handlers import RotatingFileHandler
from pathlib import Path

_ROOT_LOGGER_NAME = "novpn.desktop"
_LOG_FORMAT = "%(asctime)s %(levelname)s [%(name)s] %(message)s"
_configured_log_path: Path | None = None


def configure_logging(log_dir: Path) -> Path:
    global _configured_log_path

    log_dir.mkdir(parents=True, exist_ok=True)
    log_path = log_dir / "desktop-client.log"
    if _configured_log_path == log_path:
        return log_path

    logger = logging.getLogger(_ROOT_LOGGER_NAME)
    logger.handlers.clear()
    logger.setLevel(_resolve_log_level())
    logger.propagate = False

    formatter = logging.Formatter(_LOG_FORMAT)
    file_handler = RotatingFileHandler(
        log_path,
        maxBytes=1_048_576,
        backupCount=5,
        encoding="utf-8",
    )
    file_handler.setFormatter(formatter)
    logger.addHandler(file_handler)

    if not getattr(sys, "frozen", False):
        stream_handler = logging.StreamHandler()
        stream_handler.setFormatter(formatter)
        logger.addHandler(stream_handler)

    logging.captureWarnings(True)
    _install_exception_hooks(logger)
    _configured_log_path = log_path
    logger.info("logging initialized at %s", log_path)
    return log_path


def get_logger(name: str = "") -> logging.Logger:
    root = logging.getLogger(_ROOT_LOGGER_NAME)
    return root if not name else root.getChild(name)


def _install_exception_hooks(logger: logging.Logger) -> None:
    def handle_exception(
        exc_type: type[BaseException],
        exc_value: BaseException,
        exc_traceback,
    ) -> None:
        if issubclass(exc_type, KeyboardInterrupt):
            sys.__excepthook__(exc_type, exc_value, exc_traceback)
            return
        logger.exception(
            "unhandled exception",
            exc_info=(exc_type, exc_value, exc_traceback),
        )

    sys.excepthook = handle_exception

    if hasattr(threading, "excepthook"):
        def handle_thread_exception(args: threading.ExceptHookArgs) -> None:
            if args.exc_type is KeyboardInterrupt:
                return
            logger.exception(
                "unhandled thread exception in %s",
                getattr(args.thread, "name", "unknown"),
                exc_info=(args.exc_type, args.exc_value, args.exc_traceback),
            )

        threading.excepthook = handle_thread_exception


def _resolve_log_level() -> int:
    raw_level = os.environ.get("NOVPN_DESKTOP_LOG_LEVEL", "").strip().upper()
    if raw_level:
        resolved = getattr(logging, raw_level, None)
        if isinstance(resolved, int):
            return resolved
    return logging.INFO
