"""
=============================================================================
Personal AI Operating System - Memory Consolidation Service
=============================================================================
Background job that:
- Prunes old embeddings
- Summarizes conversation history
- Consolidates redundant memories
- Archives low-importance data

Prevents unbounded memory growth.
"""

import os
import logging
from datetime import datetime, timedelta
from contextlib import asynccontextmanager
import asyncio

from fastapi import FastAPI
from pydantic import BaseModel
import asyncpg
import redis.asyncio as redis

# Configuration
class Settings:
    DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://ai_user:change_me_in_production@memory-db:5432/ai_memory")
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    
    # Consolidation settings
    CONSOLIDATION_INTERVAL = int(os.getenv("CONSOLIDATION_INTERVAL", "3600"))  # 1 hour
    EMBEDDING_RETENTION_DAYS = int(os.getenv("EMBEDDING_RETENTION_DAYS", "90"))
    CONVERSATION_SUMMARY_AGE_DAYS = int(os.getenv("CONVERSATION_SUMMARY_AGE_DAYS", "7"))
    MAX_EMBEDDINGS_PER_USER = int(os.getenv("MAX_EMBEDDINGS_PER_USER", "10000"))

settings = Settings()

logging.basicConfig(level=getattr(logging, settings.LOG_LEVEL))
logger = logging.getLogger("memory-consolidation")

# Models
class ConsolidationStats(BaseModel):
    embeddings_pruned: int
    conversations_summarized: int
    facts_archived: int
    duration_seconds: float

class HealthResponse(BaseModel):
    status: str
    last_consolidation: str = None
    next_consolidation: str = None
    timestamp: str

# Consolidation Logic
class MemoryConsolidator:
    """Handles memory lifecycle management."""
    
    def __init__(self, db_pool: asyncpg.Pool, redis_client):
        self.db = db_pool
        self.redis = redis_client
    
    async def prune_old_embeddings(self) -> int:
        """Remove embeddings older than retention period."""
        cutoff = datetime.utcnow() - timedelta(days=settings.EMBEDDING_RETENTION_DAYS)
        
        result = await self.db.execute("""
            DELETE FROM memory_embeddings 
            WHERE created_at < $1 
            AND importance_score < 0.5
        """, cutoff)
        
        # Parse "DELETE X" to get count
        count = int(result.split()[-1]) if result else 0
        logger.info(f"Pruned {count} old embeddings")
        return count
    
    async def enforce_per_user_limits(self) -> int:
        """Ensure no user exceeds max embeddings."""
        pruned = 0
        
        # Get users over limit
        users = await self.db.fetch("""
            SELECT user_id, COUNT(*) as cnt 
            FROM memory_embeddings 
            GROUP BY user_id 
            HAVING COUNT(*) > $1
        """, settings.MAX_EMBEDDINGS_PER_USER)
        
        for user in users:
            excess = user['cnt'] - settings.MAX_EMBEDDINGS_PER_USER
            
            # Delete oldest, lowest importance first
            await self.db.execute("""
                DELETE FROM memory_embeddings 
                WHERE id IN (
                    SELECT id FROM memory_embeddings 
                    WHERE user_id = $1 
                    ORDER BY importance_score ASC, created_at ASC 
                    LIMIT $2
                )
            """, user['user_id'], excess)
            
            pruned += excess
            logger.info(f"Pruned {excess} embeddings for user {user['user_id']}")
        
        return pruned
    
    async def summarize_old_conversations(self) -> int:
        """Create summaries for old conversations and archive messages."""
        cutoff = datetime.utcnow() - timedelta(days=settings.CONVERSATION_SUMMARY_AGE_DAYS)
        
        # Find conversations to summarize
        conversations = await self.db.fetch("""
            SELECT DISTINCT conversation_id, user_id
            FROM messages 
            WHERE created_at < $1
            AND conversation_id NOT IN (
                SELECT DISTINCT conversation_id FROM conversation_summaries
            )
            LIMIT 100
        """, cutoff)
        
        summarized = 0
        for conv in conversations:
            # Get message count (we'd call LLM to summarize in production)
            messages = await self.db.fetch("""
                SELECT role, content FROM messages 
                WHERE conversation_id = $1 
                ORDER BY created_at
            """, conv['conversation_id'])
            
            if len(messages) < 3:
                continue
            
            # Create simple extractive summary (placeholder - would use LLM)
            summary_text = f"Conversation with {len(messages)} messages"
            
            await self.db.execute("""
                INSERT INTO conversation_summaries 
                (conversation_id, user_id, summary, message_count, created_at)
                VALUES ($1, $2, $3, $4, NOW())
                ON CONFLICT (conversation_id) DO UPDATE SET summary = $3
            """, conv['conversation_id'], conv['user_id'], summary_text, len(messages))
            
            summarized += 1
        
        logger.info(f"Summarized {summarized} conversations")
        return summarized
    
    async def archive_old_facts(self) -> int:
        """Move old, low-confidence facts to archive."""
        cutoff = datetime.utcnow() - timedelta(days=180)  # 6 months
        
        result = await self.db.execute("""
            UPDATE user_facts 
            SET is_active = FALSE 
            WHERE updated_at < $1 
            AND confidence < 0.5
            AND is_active = TRUE
        """, cutoff)
        
        count = int(result.split()[-1]) if result else 0
        logger.info(f"Archived {count} old facts")
        return count
    
    async def run_consolidation(self) -> ConsolidationStats:
        """Run full consolidation cycle."""
        start = datetime.utcnow()
        
        embeddings_pruned = await self.prune_old_embeddings()
        embeddings_pruned += await self.enforce_per_user_limits()
        conversations_summarized = await self.summarize_old_conversations()
        facts_archived = await self.archive_old_facts()
        
        duration = (datetime.utcnow() - start).total_seconds()
        
        stats = ConsolidationStats(
            embeddings_pruned=embeddings_pruned,
            conversations_summarized=conversations_summarized,
            facts_archived=facts_archived,
            duration_seconds=duration
        )
        
        logger.info(f"Consolidation complete: {stats}")
        return stats

# Application Lifecycle
@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("🧹 Memory Consolidation starting up...")
    
    # Initialize database pool
    try:
        app.state.db_pool = await asyncpg.create_pool(settings.DATABASE_URL, min_size=2, max_size=5)
        logger.info("✅ Database connected")
    except Exception as e:
        logger.error(f"❌ Database connection failed: {e}")
        app.state.db_pool = None
    
    # Initialize Redis
    try:
        app.state.redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        await app.state.redis.ping()
        logger.info("✅ Redis connected")
    except Exception as e:
        logger.warning(f"⚠️ Redis unavailable: {e}")
        app.state.redis = None
    
    # Initialize consolidator
    if app.state.db_pool:
        app.state.consolidator = MemoryConsolidator(app.state.db_pool, app.state.redis)
    else:
        app.state.consolidator = None
    
    app.state.last_consolidation = None
    
    # Start consolidation loop
    app.state.consolidation_task = asyncio.create_task(consolidation_loop(app))
    
    yield
    
    logger.info("🛑 Memory Consolidation shutting down...")
    app.state.consolidation_task.cancel()
    if app.state.db_pool:
        await app.state.db_pool.close()
    if app.state.redis:
        await app.state.redis.close()

async def consolidation_loop(app: FastAPI):
    """Background loop for memory consolidation with backoff."""
    logger.info("Consolidation loop started")
    
    consecutive_failures = 0
    max_backoff = 3600  # 1 hour max
    base_interval = settings.CONSOLIDATION_INTERVAL
    
    # Initial delay to let system stabilize
    await asyncio.sleep(60)
    
    while True:
        try:
            if not app.state.consolidator:
                consecutive_failures += 1
                backoff = min(base_interval * (2 ** min(consecutive_failures, 4)), max_backoff)
                logger.warning(f"Consolidator unavailable, backing off for {backoff}s")
                await asyncio.sleep(backoff)
                continue
            
            stats = await app.state.consolidator.run_consolidation()
            app.state.last_consolidation = datetime.utcnow().isoformat()
            
            # Store stats in Redis
            if app.state.redis:
                import json
                await app.state.redis.set(
                    "consolidation:last_stats",
                    json.dumps(stats.model_dump()),
                    ex=86400  # 24 hour TTL
                )
            
            # Reset backoff on success
            if consecutive_failures > 0:
                logger.info(f"Consolidation recovered after {consecutive_failures} failures")
            consecutive_failures = 0
            
            await asyncio.sleep(base_interval)
            
        except asyncio.CancelledError:
            break
        except Exception as e:
            consecutive_failures += 1
            backoff = min(base_interval * (2 ** min(consecutive_failures, 4)), max_backoff)
            logger.error(f"Consolidation error (attempt {consecutive_failures}): {e}")
            await asyncio.sleep(backoff)

# FastAPI Application
app = FastAPI(
    title="Personal AI OS - Memory Consolidation",
    description="Memory Lifecycle Management",
    version="1.0.0",
    lifespan=lifespan
)

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    next_run = None
    if app.state.last_consolidation:
        last = datetime.fromisoformat(app.state.last_consolidation)
        next_run = (last + timedelta(seconds=settings.CONSOLIDATION_INTERVAL)).isoformat()
    
    return HealthResponse(
        status="healthy" if app.state.consolidator else "degraded",
        last_consolidation=app.state.last_consolidation,
        next_consolidation=next_run,
        timestamp=datetime.utcnow().isoformat()
    )

@app.post("/consolidate")
async def trigger_consolidation():
    """Manually trigger consolidation."""
    if not app.state.consolidator:
        from fastapi import HTTPException
        raise HTTPException(status_code=503, detail="Consolidator unavailable")
    
    stats = await app.state.consolidator.run_consolidation()
    app.state.last_consolidation = datetime.utcnow().isoformat()
    
    return {"status": "completed", "stats": stats.model_dump()}

@app.get("/stats")
async def get_stats():
    """Get last consolidation stats."""
    if not app.state.redis:
        return {"stats": None}
    
    import json
    data = await app.state.redis.get("consolidation:last_stats")
    return {"stats": json.loads(data) if data else None}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8009)
