"""端口映射配置的 JSON 持久化（线程安全、原子写入）。"""
import json
import os
import threading
from datetime import datetime, timezone
from typing import Optional

from .config import settings


def now_iso() -> str:
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


class Store:
    def __init__(self, path: str) -> None:
        self.path = path
        self._lock = threading.Lock()
        self._data = {"mappings": []}
        self._load()

    def _load(self) -> None:
        try:
            with open(self.path, "r", encoding="utf-8") as fh:
                data = json.load(fh)
            if isinstance(data, dict) and isinstance(data.get("mappings"), list):
                self._data = data
        except FileNotFoundError:
            self._data = {"mappings": []}
        except (json.JSONDecodeError, OSError):
            self._data = {"mappings": []}

    def _save(self) -> None:
        directory = os.path.dirname(self.path)
        if directory:
            os.makedirs(directory, exist_ok=True)
        tmp = self.path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as fh:
            json.dump(self._data, fh, ensure_ascii=False, indent=2)
        os.replace(tmp, self.path)

    def all(self) -> list:
        with self._lock:
            return [dict(m) for m in self._data["mappings"]]

    def get(self, mapping_id: str) -> Optional[dict]:
        with self._lock:
            for m in self._data["mappings"]:
                if m["id"] == mapping_id:
                    return dict(m)
        return None

    def add(self, entry: dict) -> dict:
        with self._lock:
            self._data["mappings"].append(entry)
            self._save()
            return dict(entry)

    def remove(self, mapping_id: str) -> bool:
        with self._lock:
            before = len(self._data["mappings"])
            self._data["mappings"] = [
                m for m in self._data["mappings"] if m["id"] != mapping_id
            ]
            if len(self._data["mappings"]) == before:
                return False
            self._save()
            return True


_store: Optional[Store] = None


def get_store() -> Store:
    global _store
    if _store is None:
        _store = Store(settings.config_path)
    return _store
