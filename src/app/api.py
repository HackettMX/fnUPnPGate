"""REST API 路由：状态、端口映射增删查、网关重新发现。"""
import uuid

from fastapi import APIRouter, HTTPException

from . import schemas
from .store import get_store, now_iso
from .upnp import UPnPError, detect_lan_ip, get_manager

router = APIRouter(prefix="/api")


def _to_item(entry: dict, active: bool, managed: bool = True) -> schemas.MappingItem:
    return schemas.MappingItem(
        id=entry["id"],
        description=entry.get("description", ""),
        protocol=entry.get("protocol", "TCP"),
        external_port=int(entry["external_port"]),
        internal_port=int(entry["internal_port"]),
        internal_client=entry.get("internal_client", ""),
        lease_time=int(entry.get("lease_time", 0)),
        active=active,
        managed=managed,
        created_at=entry.get("created_at", ""),
    )


def _from_router(mapping) -> schemas.MappingItem:
    internal_client = getattr(mapping, "internal_client", None)
    internal_client = str(internal_client) if internal_client is not None else ""
    lease_duration = getattr(mapping, "lease_duration", None)
    lease_time = int(lease_duration.total_seconds()) if lease_duration is not None else 0
    return schemas.MappingItem(
        id="ext-" + uuid.uuid4().hex[:12],
        description=getattr(mapping, "description", "") or "",
        protocol=getattr(mapping, "protocol", "TCP"),
        external_port=int(getattr(mapping, "external_port", 0) or 0),
        internal_port=int(getattr(mapping, "internal_port", 0) or 0),
        internal_client=internal_client,
        lease_time=lease_time,
        active=True,
        managed=False,
        created_at="",
    )


@router.get("/health")
async def health():
    return {"status": "ok"}


@router.get("/status", response_model=schemas.StatusResponse)
async def status(force: bool = False):
    info = await get_manager().status(force=force)
    return schemas.StatusResponse(
        connected=info.connected,
        gateway_ip=info.gateway_ip,
        external_ip=info.external_ip,
        model_name=info.model_name,
        manufacturer=info.manufacturer,
        service_type=info.service_type,
        error=info.error,
    )


@router.post("/discover", response_model=schemas.StatusResponse)
async def discover():
    info = await get_manager().discover(force=True)
    return schemas.StatusResponse(
        connected=info.connected,
        gateway_ip=info.gateway_ip,
        external_ip=info.external_ip,
        model_name=info.model_name,
        manufacturer=info.manufacturer,
        service_type=info.service_type,
        error=info.error,
    )


@router.get("/mappings", response_model=schemas.MappingList)
async def list_mappings():
    store = get_store()
    mgr = get_manager()
    managed = store.all()
    lan_ip = detect_lan_ip()

    info = await mgr.status()
    resp = schemas.MappingList(
        connected=info.connected,
        lan_ip=lan_ip,
        gateway_ip=info.gateway_ip,
        external_ip=info.external_ip,
        error=info.error,
    )

    if not info.connected:
        resp.mappings = [_to_item(m, active=False) for m in managed]
        return resp

    # 枚举路由器 NAT 表
    try:
        router_entries = await mgr.list_mappings()
        enumerate_ok = True
    except UPnPError:
        router_entries = []
        enumerate_ok = False

    router_keys = {
        (int(getattr(m, "external_port", 0) or 0), getattr(m, "protocol", ""))
        for m in router_entries
    }
    managed_keys = {
        (int(m["external_port"]), m["protocol"]) for m in managed
    }

    items = []
    for m in managed:
        key = (int(m["external_port"]), m["protocol"])
        active = key in router_keys
        if not enumerate_ok and not active:
            try:
                active = (
                    await mgr.get_specific(m["external_port"], m["protocol"]) is not None
                )
            except Exception:
                active = False
        items.append(_to_item(m, active=active))

    # 路由器上存在、但并非本应用管理的映射
    for m in router_entries:
        key = (int(getattr(m, "external_port", 0) or 0), getattr(m, "protocol", ""))
        if key not in managed_keys:
            items.append(_from_router(m))

    resp.mappings = items
    return resp


@router.post("/mappings", response_model=schemas.MappingItem, status_code=201)
async def create_mapping(payload: schemas.MappingCreate):
    mgr = get_manager()
    store = get_store()

    internal_client = payload.internal_client or detect_lan_ip()
    if not internal_client:
        raise HTTPException(status_code=400, detail="无法确定本机局域网 IP，请手动填写内部 IP")

    # 冲突检查（路由器已有该外部端口+协议）
    try:
        existing = await mgr.get_specific(payload.external_port, payload.protocol)
    except UPnPError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    if existing is not None:
        raise HTTPException(
            status_code=409,
            detail=f"外部端口 {payload.external_port}/{payload.protocol} 已被占用",
        )

    description = payload.description.strip() or f"端口 {payload.external_port}"
    try:
        await mgr.add_mapping(
            external_port=payload.external_port,
            protocol=payload.protocol,
            internal_port=payload.internal_port,
            internal_client=internal_client,
            description=description,
            lease_time=payload.lease_time,
        )
    except UPnPError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc

    entry = {
        "id": uuid.uuid4().hex,
        "description": description,
        "protocol": payload.protocol,
        "external_port": payload.external_port,
        "internal_port": payload.internal_port,
        "internal_client": internal_client,
        "lease_time": payload.lease_time,
        "created_at": now_iso(),
    }
    saved = store.add(entry)
    return _to_item(saved, active=True)


@router.delete("/mappings/{mapping_id}", status_code=204)
async def delete_mapping(mapping_id: str):
    store = get_store()
    entry = store.get(mapping_id)
    if entry is None:
        raise HTTPException(status_code=404, detail="端口映射不存在")

    try:
        await get_manager().delete_mapping(entry["external_port"], entry["protocol"])
    except UPnPError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc

    store.remove(mapping_id)


@router.delete("/mappings/port/{external_port}/{protocol}", status_code=204)
async def delete_mapping_by_port(external_port: int, protocol: str):
    """按外部端口+协议直接删除路由器映射（用于清理非本应用管理的外部条目）。"""
    if protocol not in ("TCP", "UDP"):
        raise HTTPException(status_code=400, detail="协议必须是 TCP 或 UDP")
    try:
        await get_manager().delete_mapping(external_port, protocol)
    except UPnPError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    store = get_store()
    for m in store.all():
        if int(m["external_port"]) == external_port and m["protocol"] == protocol:
            store.remove(m["id"])


@router.post("/mappings/{mapping_id}/enable", response_model=schemas.MappingItem)
async def enable_mapping(mapping_id: str):
    store = get_store()
    entry = store.get(mapping_id)
    if entry is None:
        raise HTTPException(status_code=404, detail="端口映射不存在")

    try:
        await get_manager().add_mapping(
            external_port=entry["external_port"],
            protocol=entry["protocol"],
            internal_port=entry["internal_port"],
            internal_client=entry["internal_client"],
            description=entry.get("description", ""),
            lease_time=entry.get("lease_time", 0),
        )
    except UPnPError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc

    return _to_item(entry, active=True)
