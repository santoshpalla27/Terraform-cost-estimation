"""
=============================================================================
Personal AI Operating System - Info Engine
=============================================================================
Layer 4: Autonomous Systems - Background Intelligence

The Info Engine monitors external information sources.
- News ingestion
- RSS feeds
- API polling
- Topic monitoring
- Interest tracking

This is how the AI "keeps you updated".
"""

import os
import logging
from datetime import datetime
from contextlib import asynccontextmanager
from typing import Optional, List, Dict
import asyncio

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import httpx
import redis.asyncio as redis
import feedparser

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    MEMORY_URL = os.getenv("MEMORY_URL", "http://memory-service:8002")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    POLL_INTERVAL = int(os.getenv("POLL_INTERVAL", "300"))  # 5 minutes

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("info-engine")

# =============================================================================
# Models
# =============================================================================

class FeedSource(BaseModel):
    id: Optional[str] = None
    name: str
    url: str
    type: str = "rss"  # rss, api
    enabled: bool = True
    check_interval: int = 300  # seconds

class NewsItem(BaseModel):
    id: str
    title: str
    summary: Optional[str]
    url: Optional[str]
    source: str
    published_at: Optional[str]
    relevance_score: float = 0.0

class InterestTopic(BaseModel):
    topic: str
    keywords: List[str] = []
    priority: int = 5
    user_id: str = "primary"

class HealthResponse(BaseModel):
    status: str
    feeds_monitored: int
    last_poll: Optional[str]
    timestamp: str

# =============================================================================
# Feed Processor
# =============================================================================

class FeedProcessor:
    """Process RSS feeds and extract news items."""
    
    def __init__(self, http_client: httpx.AsyncClient):
        self.http = http_client
    
    async def fetch_rss(self, url: str) -> List[Dict]:
        """Fetch and parse RSS feed."""
        try:
            response = await self.http.get(url, timeout=30.0)
            feed = feedparser.parse(response.text)
            
            items = []
            for entry in feed.entries[:20]:  # Limit to 20 items
                items.append({
                    "id": entry.get("id", entry.get("link", "")),
                    "title": entry.get("title", ""),
                    "summary": entry.get("summary", "")[:500] if entry.get("summary") else None,
                    "url": entry.get("link", ""),
                    "published": entry.get("published", ""),
                    "source": feed.feed.get("title", url)
                })
            
            return items
            
        except Exception as e:
            logger.error(f"Failed to fetch RSS {url}: {e}")
            return []
    
    def calculate_relevance(self, item: Dict, interests: List[Dict]) -> float:
        """Calculate relevance score based on user interests."""
        if not interests:
            return 0.5
        
        text = (item.get("title", "") + " " + (item.get("summary") or "")).lower()
        
        max_score = 0.0
        for interest in interests:
            keywords = interest.get("keywords", [])
            topic = interest.get("topic", "")
            priority = interest.get("priority", 5) / 10.0
            
            # Check topic match
            if topic.lower() in text:
                max_score = max(max_score, 0.8 * priority)
            
            # Check keyword matches
            keyword_matches = sum(1 for kw in keywords if kw.lower() in text)
            if keywords:
                keyword_score = (keyword_matches / len(keywords)) * priority
                max_score = max(max_score, keyword_score)
        
        return min(max_score, 1.0)

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("📰 Info Engine starting up...")
    
    # Initialize Redis
    try:
        app.state.redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        await app.state.redis.ping()
        logger.info("✅ Redis connected")
    except Exception as e:
        logger.warning(f"⚠️ Redis unavailable: {e}")
        app.state.redis = None
    
    # Initialize HTTP client
    app.state.http_client = httpx.AsyncClient(timeout=60.0)
    app.state.feed_processor = FeedProcessor(app.state.http_client)
    app.state.last_poll = None
    
    # Start polling loop
    app.state.poll_task = asyncio.create_task(poll_loop(app))
    
    yield
    
    logger.info("🛑 Info Engine shutting down...")
    app.state.poll_task.cancel()
    await app.state.http_client.aclose()
    if app.state.redis:
        await app.state.redis.close()

async def poll_loop(app: FastAPI):
    """Background loop for polling feeds with adaptive backoff."""
    logger.info("Poll loop started")
    
    # Adaptive backoff state
    consecutive_failures = 0
    max_backoff = 1800  # 30 minutes max
    base_interval = settings.POLL_INTERVAL
    
    while True:
        try:
            if not app.state.redis:
                # Redis unavailable - exponential backoff
                consecutive_failures += 1
                backoff = min(base_interval * (2 ** min(consecutive_failures, 5)), max_backoff)
                logger.warning(f"Redis unavailable, backing off for {backoff}s (failures: {consecutive_failures})")
                await asyncio.sleep(backoff)
                continue
            
            await poll_feeds(app)
            app.state.last_poll = datetime.utcnow().isoformat()
            
            # Reset backoff on success
            if consecutive_failures > 0:
                logger.info(f"Info engine recovered after {consecutive_failures} failures")
            consecutive_failures = 0
            
            await asyncio.sleep(base_interval)
            
        except asyncio.CancelledError:
            break
        except Exception as e:
            consecutive_failures += 1
            backoff = min(base_interval * (2 ** min(consecutive_failures, 5)), max_backoff)
            logger.error(f"Poll error (attempt {consecutive_failures}): {e}")
            await asyncio.sleep(backoff)

async def poll_feeds(app: FastAPI):
    """Poll all configured feeds."""
    import json
    
    # Get configured feeds
    feeds_data = await app.state.redis.hgetall("info:feeds")
    feeds = [json.loads(v) for v in feeds_data.values()]
    
    if not feeds:
        return
    
    # Get user interests
    interests_data = await app.state.redis.hgetall("info:interests")
    interests = [json.loads(v) for v in interests_data.values()]
    
    for feed in feeds:
        if not feed.get("enabled", True):
            continue
        
        logger.info(f"Polling feed: {feed.get('name')}")
        
        items = await app.state.feed_processor.fetch_rss(feed.get("url"))
        
        for item in items:
            # Check if already processed
            item_key = f"info:seen:{item['id']}"
            if await app.state.redis.exists(item_key):
                continue
            
            # Calculate relevance
            relevance = app.state.feed_processor.calculate_relevance(item, interests)
            item["relevance_score"] = relevance
            
            # Store if relevant
            if relevance >= 0.3:
                await app.state.redis.lpush(
                    "info:news_queue",
                    json.dumps(item)
                )
                await app.state.redis.ltrim("info:news_queue", 0, 99)
            
            # Mark as seen
            await app.state.redis.setex(item_key, 86400 * 7, "1")  # 7 day TTL

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - Info Engine",
    description="Background Intelligence",
    version="1.0.0",
    lifespan=lifespan
)

# =============================================================================
# Endpoints
# =============================================================================

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    feed_count = 0
    if app.state.redis:
        feed_count = await app.state.redis.hlen("info:feeds")
    
    return HealthResponse(
        status="healthy",
        feeds_monitored=feed_count,
        last_poll=app.state.last_poll,
        timestamp=datetime.utcnow().isoformat()
    )

@app.post("/feeds")
async def add_feed(feed: FeedSource):
    """Add a news feed source."""
    if not app.state.redis:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    import uuid
    import json
    
    feed_id = feed.id or str(uuid.uuid4())
    feed_data = feed.model_dump()
    feed_data["id"] = feed_id
    
    await app.state.redis.hset("info:feeds", feed_id, json.dumps(feed_data))
    
    return {"status": "added", "feed_id": feed_id}

@app.get("/feeds")
async def list_feeds():
    """List configured feeds."""
    if not app.state.redis:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    import json
    feeds_data = await app.state.redis.hgetall("info:feeds")
    feeds = [json.loads(v) for v in feeds_data.values()]
    
    return {"feeds": feeds}

@app.delete("/feeds/{feed_id}")
async def delete_feed(feed_id: str):
    """Delete a feed."""
    if not app.state.redis:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    await app.state.redis.hdel("info:feeds", feed_id)
    return {"status": "deleted"}

@app.post("/interests")
async def add_interest(interest: InterestTopic):
    """Add a topic of interest."""
    if not app.state.redis:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    import uuid
    import json
    
    interest_id = str(uuid.uuid4())
    await app.state.redis.hset(
        "info:interests",
        interest_id,
        json.dumps(interest.model_dump())
    )
    
    return {"status": "added", "interest_id": interest_id}

@app.get("/interests")
async def list_interests():
    """List configured interests."""
    if not app.state.redis:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    import json
    data = await app.state.redis.hgetall("info:interests")
    interests = [json.loads(v) for v in data.values()]
    
    return {"interests": interests}

@app.get("/news")
async def get_news(limit: int = 20, min_relevance: float = 0.0):
    """Get recent news items."""
    if not app.state.redis:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    import json
    
    items_raw = await app.state.redis.lrange("info:news_queue", 0, limit - 1)
    items = [json.loads(i) for i in items_raw]
    
    if min_relevance > 0:
        items = [i for i in items if i.get("relevance_score", 0) >= min_relevance]
    
    return {"news": items}

@app.post("/poll")
async def trigger_poll():
    """Manually trigger feed polling."""
    if not app.state.redis:
        raise HTTPException(status_code=503, detail="Service unavailable")
    
    await poll_feeds(app)
    
    return {"status": "polled", "timestamp": datetime.utcnow().isoformat()}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8007)
