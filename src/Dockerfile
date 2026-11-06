# fnPortGate（端口转发）— 基于 UPnP 的路由器端口转发管理
FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app
COPY static ./static

# 端口转发应用必须以 host 网络运行（见 docker-compose），
# 容器内进程无需 root 也能完成 UPnP 调用与写数据目录。
ENV FN_UPNP_DATA_DIR=/data \
    FN_UPNP_WEB_PORT=8090

EXPOSE 8090

CMD ["sh", "-c", "exec uvicorn app.main:app --host 0.0.0.0 --port ${FN_UPNP_WEB_PORT:-8090}"]
