"""集中配置：全部来自环境变量，启动时快速失败。"""
import os


def _int(name: str, default: int) -> int:
    raw = os.environ.get(name)
    if raw is None or raw == "":
        return default
    try:
        return int(raw)
    except ValueError as exc:
        raise ValueError(f"环境变量 {name} 必须是整数，当前值: {raw!r}") from exc


def _str(name: str, default: str = "") -> str:
    return os.environ.get(name, default)


class Settings:
    """应用配置。密钥一律通过环境变量注入，不写入代码库。"""

    def __init__(self) -> None:
        self.web_host = _str("FN_UPNP_WEB_HOST", "0.0.0.0")
        self.web_port = _int("FN_UPNP_WEB_PORT", 8090)
        self.data_dir = _str("FN_UPNP_DATA_DIR", "/data")
        self.log_level = _str("FN_UPNP_LOG_LEVEL", "INFO").upper()
        # SSDP 发现网关的等待秒数
        self.discovery_timeout = _int("FN_UPNP_DISCOVERY_TIMEOUT", 5)
        # 网关信息缓存秒数
        self.discovery_ttl = _int("FN_UPNP_DISCOVERY_TTL", 300)
        # 新增端口映射的默认租期（秒，0 表示永久）
        self.default_lease_time = _int("FN_UPNP_DEFAULT_LEASE_TIME", 0)

    @property
    def config_path(self) -> str:
        return os.path.join(self.data_dir, "fn-upnp.json")


settings = Settings()
