"""接口输入/输出模型（边界校验，不信任客户端数据）。"""
from typing import Literal, Optional

from pydantic import BaseModel, Field


class MappingCreate(BaseModel):
    description: str = Field(default="", max_length=128)
    protocol: Literal["TCP", "UDP"] = "TCP"
    external_port: int = Field(ge=1, le=65535)
    internal_port: int = Field(ge=1, le=65535)
    internal_client: Optional[str] = Field(default=None, max_length=64)
    lease_time: int = Field(default=0, ge=0, le=86400 * 365)


class MappingItem(BaseModel):
    id: str
    description: str
    protocol: str
    external_port: int
    internal_port: int
    internal_client: str
    lease_time: int
    active: bool
    managed: bool
    created_at: str


class MappingList(BaseModel):
    connected: bool = False
    lan_ip: Optional[str] = None
    gateway_ip: Optional[str] = None
    external_ip: Optional[str] = None
    error: Optional[str] = None
    mappings: list[MappingItem] = []


class StatusResponse(BaseModel):
    connected: bool = False
    gateway_ip: Optional[str] = None
    external_ip: Optional[str] = None
    model_name: Optional[str] = None
    manufacturer: Optional[str] = None
    service_type: Optional[str] = None
    error: Optional[str] = None


class MessageResponse(BaseModel):
    message: str
