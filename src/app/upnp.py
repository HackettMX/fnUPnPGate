"""UPnP IGD 网关封装：SSDP 发现、公网 IP、端口映射增删查。

基于 async-upnp-client（>=0.42,<0.50）新版 API：
- 设备创建：UpnpFactory(async_create_device) -> UpnpDevice
- IGD 配置：IgdDevice(device, event_handler=None)
- 发现：async_upnp_client.search.async_search（回调收集 + is_profile_device 过滤）
"""
import asyncio
import logging
import socket
import time
from dataclasses import dataclass
from typing import Optional
from urllib.parse import urlparse

from async_upnp_client.aiohttp import AiohttpRequester
from async_upnp_client.client_factory import UpnpFactory
from async_upnp_client.profiles.igd import IgdDevice
from async_upnp_client.search import async_search as ssdp_search

from .config import settings

log = logging.getLogger("fnupnp.upnp")

MAX_MAPPINGS = 256


class UPnPError(Exception):
    """业务层 UPnP 错误，message 为面向用户的文案。"""


def detect_lan_ip() -> Optional[str]:
    """通过 UDP 连接探测本机局域网 IP（不真正发包，无需外网）。"""
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        try:
            s.connect(("223.5.5.5", 53))
            return s.getsockname()[0]
        finally:
            s.close()
    except OSError:
        return None


@dataclass
class GatewayInfo:
    connected: bool = False
    gateway_ip: Optional[str] = None
    external_ip: Optional[str] = None
    model_name: Optional[str] = None
    manufacturer: Optional[str] = None
    service_type: Optional[str] = None
    error: Optional[str] = None


def friendly_error(exc: Exception) -> str:
    """把底层异常映射为用户可读文案，绝不泄露堆栈。"""
    code = getattr(exc, "error_code", None) or getattr(exc, "code", None)
    text = str(exc) or exc.__class__.__name__

    code_map = {
        402: "路由器返回参数错误（Invalid Args）",
        501: "路由器拒绝了操作，可能 UPnP 被禁用或权限不足",
        715: "通配符 IP 不被允许",
        718: "该外部端口已被其他映射占用（ConflictInMappingEntry）",
        724: "内部端口与外部端口不一致不被允许",
        725: "路由器仅支持永久租期",
        728: "路由器无可用端口映射资源（NoPortMapsAvailable）",
    }
    if code in code_map:
        return code_map[code]

    if isinstance(exc, (asyncio.TimeoutError, TimeoutError)):
        return "连接路由器超时，请检查网络或路由器 UPnP 状态"
    if "未发现" in text:
        return text
    if "Cannot assign requested address" in text or "Connect call failed" in text:
        return "无法连接路由器网关，请检查网络连通性"
    return f"操作失败：{text}"


class UPnPManager:
    """管理 IGD 网关的发现与端口映射操作（懒发现 + TTL 缓存）。"""

    def __init__(self) -> None:
        self._requester: Optional[AiohttpRequester] = None
        self._factory: Optional[UpnpFactory] = None
        self._igd: Optional[IgdDevice] = None
        self._gateway_ip: Optional[str] = None
        self._service_type: Optional[str] = None
        self._discovered_at: float = 0.0
        self._lock = asyncio.Lock()

    # ---- 生命周期 ----

    async def close(self) -> None:
        if self._requester is not None:
            closer = getattr(self._requester, "async_close", None)
            if closer is not None:
                try:
                    await closer()
                except Exception:
                    pass
        self._requester = None
        self._factory = None
        self._igd = None

    def _fresh(self) -> bool:
        return (time.monotonic() - self._discovered_at) < settings.discovery_ttl

    def _ready(self) -> bool:
        return self._igd is not None

    # ---- 发现 ----

    async def discover(self, force: bool = False) -> GatewayInfo:
        if not force and self._ready() and self._fresh():
            return await self._snapshot()

        async with self._lock:
            if not force and self._ready() and self._fresh():
                return await self._snapshot()

            await self.close()
            try:
                await self._do_discover()
            except Exception as exc:  # noqa: BLE001 - 顶层兜底
                log.warning("UPnP discovery failed: %s", exc)
                return GatewayInfo(connected=False, error=friendly_error(exc))
            self._discovered_at = time.monotonic()
            return await self._snapshot()

    async def _do_discover(self) -> None:
        requester = AiohttpRequester(timeout=10)
        factory = UpnpFactory(requester, non_strict=True)
        self._requester = requester
        self._factory = factory

        locations: dict = {}

        async def on_response(data) -> None:
            if "location" in data:
                locations.setdefault(data["location"], True)

        await ssdp_search(
            async_callback=on_response,
            timeout=settings.discovery_timeout,
        )

        for location in locations:
            try:
                device = await factory.async_create_device(location)
            except Exception:
                continue
            if not IgdDevice.is_profile_device(device):
                continue
            igd = IgdDevice(device, None)
            # 验证可用：取一次公网 IP，失败视为不可用
            await igd.async_get_external_ip_address()
            self._igd = igd
            self._gateway_ip = urlparse(location).hostname
            self._service_type = "WANIPConnection"
            return

        raise UPnPError(
            "未发现支持 UPnP 的路由器。请确认路由器已开启 UPnP，且本机与路由器在同一局域网。"
        )

    async def _snapshot(self) -> GatewayInfo:
        info = GatewayInfo(
            connected=self._ready(),
            gateway_ip=self._gateway_ip,
            service_type=self._service_type,
        )
        if not self._ready():
            return info
        try:
            info.external_ip = await self._igd.async_get_external_ip_address()
            info.model_name = self._igd.model_name or None
            info.manufacturer = self._igd.manufacturer or None
        except Exception as exc:
            info.connected = False
            info.error = friendly_error(exc)
            self._igd = None  # 失效，下次重新发现
        return info

    # ---- 操作 ----

    async def _ensure_ready(self) -> None:
        if not self._ready() or not self._fresh():
            info = await self.discover()
            if not info.connected or not self._ready():
                raise UPnPError(info.error or "路由器未连接")

    def _action(self, action_name: str):
        """获取 WANIP/WANPPP 的 UPnP 动作（优先 WANIP）。"""
        return self._igd._any_action(["WANIPC", "WANPPP"], action_name)

    async def status(self, force: bool = False) -> GatewayInfo:
        return await self.discover(force=force)

    async def list_mappings(self) -> list:
        await self._ensure_ready()
        try:
            return await self._enumerate()
        except UPnPError:
            raise
        except Exception as exc:
            raise UPnPError(friendly_error(exc)) from exc

    async def _enumerate(self) -> list:
        igd = self._igd
        try:
            count = await igd.async_get_port_mapping_number_of_entries()
            count = int(count) if count is not None else MAX_MAPPINGS
        except Exception:
            count = MAX_MAPPINGS
        count = max(0, min(count, MAX_MAPPINGS))

        out = []
        for index in range(count):
            try:
                entry = await igd.async_get_generic_port_mapping_entry(index)
            except Exception:
                break
            if entry is None:
                break
            out.append(entry)
        return out

    async def get_specific(self, external_port: int, protocol: str):
        await self._ensure_ready()
        try:
            return await self._igd.async_get_specific_port_mapping_entry(
                remote_host=None, external_port=external_port, protocol=protocol
            )
        except Exception:
            return None

    async def add_mapping(
        self,
        external_port: int,
        protocol: str,
        internal_port: int,
        internal_client: str,
        description: str,
        lease_time: int,
    ) -> None:
        await self._ensure_ready()
        action = self._action("AddPortMapping")
        if action is None:
            raise UPnPError("路由器不支持添加端口映射")
        try:
            await action.async_call(
                NewRemoteHost="",  # 空串 = 通配，接受任意来源
                NewExternalPort=external_port,
                NewProtocol=protocol,
                NewInternalPort=internal_port,
                NewInternalClient=internal_client,
                NewEnabled="1",
                NewPortMappingDescription=description,
                NewLeaseDuration=lease_time,
            )
        except Exception as exc:
            raise UPnPError(friendly_error(exc)) from exc

    async def delete_mapping(self, external_port: int, protocol: str) -> None:
        await self._ensure_ready()
        action = self._action("DeletePortMapping")
        if action is None:
            raise UPnPError("路由器不支持删除端口映射")
        try:
            await action.async_call(
                NewRemoteHost="",
                NewExternalPort=external_port,
                NewProtocol=protocol,
            )
        except Exception as exc:
            raise UPnPError(friendly_error(exc)) from exc


_manager: Optional[UPnPManager] = None


def get_manager() -> UPnPManager:
    global _manager
    if _manager is None:
        _manager = UPnPManager()
    return _manager
