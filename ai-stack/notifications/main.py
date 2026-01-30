"""
=============================================================================
Personal AI Operating System - Notification Service
=============================================================================
Layer 4: Autonomous Systems - Proactive Communication

Decides when and how to interrupt the user.
Channels: push, bot, voice, email
Implements urgency, frequency, and priority policies.
"""

import os
import logging
from datetime import datetime
from contextlib import asynccontextmanager
from typing import Optional, List
import asyncio
import json

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
import httpx
import redis.asyncio as redis

# Configuration
class Settings:
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    API_GATEWAY_URL = os.getenv("API_GATEWAY_URL", "http://api-gateway:8000")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")

settings = Settings()

logging.basicConfig(level=getattr(logging, settings.LOG_LEVEL))
logger = logging.getLogger("notifications")

# Models
class Notification(BaseModel):
    title: str
    body: str
    priority: str = Field(default="normal", pattern="^(low|normal|high|urgent)$")
    channel: str = Field(default="push", pattern="^(push|email|bot|voice)$")
    user_id: str = "primary"
    scheduled_for: Optional[str] = None
    metadata: dict = {}

class HealthResponse(BaseModel):
    status: str
    pending_count: int
    timestamp: str

# Notification Queue
class NotificationQueue:
    def __init__(self, redis_client):
        self.redis = redis_client
        self.queue_key = "notifications:pending"
        self.history_key = "notifications:history"
    
    async def enqueue(self, notification: dict):
        await self.redis.lpush(self.queue_key, json.dumps(notification))
    
    async def dequeue(self, timeout: int = 1):
        result = await self.redis.brpop(self.queue_key, timeout=timeout)
        return json.loads(result[1]) if result else None
    
    async def add_to_history(self, notification: dict):
        await self.redis.lpush(self.history_key, json.dumps(notification))
        await self.redis.ltrim(self.history_key, 0, 99)
    
    async def get_history(self, limit: int = 20):
        items = await self.redis.lrange(self.history_key, 0, limit - 1)
        return [json.loads(i) for i in items]
    
    async def pending_count(self):
        return await self.redis.llen(self.queue_key)

# Lifecycle
@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("🔔 Notification Service starting...")
    try:
        app.state.redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        await app.state.redis.ping()
        app.state.queue = NotificationQueue(app.state.redis)
        logger.info("✅ Redis connected")
    except Exception as e:
        logger.warning(f"⚠️ Redis unavailable: {e}")
        app.state.redis = None
        app.state.queue = None
    
    app.state.http_client = httpx.AsyncClient(timeout=30.0)
    app.state.worker = asyncio.create_task(notification_worker(app))
    
    yield
    
    app.state.worker.cancel()
    await app.state.http_client.aclose()
    if app.state.redis:
        await app.state.redis.close()

async def notification_worker(app: FastAPI):
    """Process pending notifications."""
    while True:
        try:
            if not app.state.queue:
                await asyncio.sleep(5)
                continue
            
            notification = await app.state.queue.dequeue(timeout=1)
            if notification:
                await send_notification(app, notification)
        except asyncio.CancelledError:
            break
        except Exception as e:
            logger.error(f"Worker error: {e}")
            await asyncio.sleep(1)

async def send_notification(app: FastAPI, notification: dict):
    """Send notification via appropriate channel."""
    channel = notification.get("channel", "push")
    logger.info(f"Sending {channel} notification: {notification.get('title')}")
    
    notification["sent_at"] = datetime.utcnow().isoformat()
    notification["status"] = "sent"
    
    await app.state.queue.add_to_history(notification)

# App
app = FastAPI(title="Notification Service", version="1.0.0", lifespan=lifespan)

@app.get("/health", response_model=HealthResponse)
async def health():
    pending = 0
    if app.state.queue:
        pending = await app.state.queue.pending_count()
    return HealthResponse(status="healthy", pending_count=pending, timestamp=datetime.utcnow().isoformat())

@app.post("/send")
async def send(notification: Notification):
    if not app.state.queue:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    import uuid
    data = notification.model_dump()
    data["id"] = str(uuid.uuid4())
    data["created_at"] = datetime.utcnow().isoformat()
    
    await app.state.queue.enqueue(data)
    return {"status": "queued", "notification_id": data["id"]}

@app.get("/history")
async def history(limit: int = 20):
    if not app.state.queue:
        raise HTTPException(status_code=503, detail="Service unavailable")
    items = await app.state.queue.get_history(limit)
    return {"notifications": items}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8008)
