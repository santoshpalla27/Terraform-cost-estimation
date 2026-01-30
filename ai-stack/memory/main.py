"""
=============================================================================
Personal AI Operating System - Memory Service
=============================================================================
Layer 5 Interface: Persistence Layer API

The Memory Service is a knowledge engine that provides:
- Conversation storage and retrieval
- Semantic search (vector embeddings)
- Structured fact storage
- User profile management

All other services access memory through this API.
Direct database access is prohibited outside this service.
"""

import os
import logging
from datetime import datetime
from contextlib import asynccontextmanager
from typing import Optional, List
import asyncio

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
import asyncpg
import redis.asyncio as redis
import numpy as np

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://ai_user:ai_secret_password@memory-db:5432/ai_memory")
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
    EMBEDDING_DIM = 384  # Dimension for all-MiniLM-L6-v2

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("memory-service")

# =============================================================================
# Models
# =============================================================================

class Message(BaseModel):
    role: str = Field(..., pattern="^(user|assistant|system)$")
    content: str
    metadata: Optional[dict] = None

class StoreRequest(BaseModel):
    conversation_id: str
    user_id: str = "primary"
    messages: List[Message]

class SearchRequest(BaseModel):
    query: str
    user_id: str = "primary"
    limit: int = 10
    threshold: float = 0.7

class SearchResult(BaseModel):
    id: str
    content: str
    source_type: str
    similarity: float
    metadata: Optional[dict] = None

class FactRequest(BaseModel):
    user_id: str = "primary"
    category: str
    subject: str
    predicate: str
    object: str
    confidence: float = 1.0
    source: str = "explicit"

class HealthResponse(BaseModel):
    status: str
    database: str
    redis: str
    embedding_model: str
    timestamp: str

# =============================================================================
# Embedding Model (Lazy Loading)
# =============================================================================

class EmbeddingService:
    def __init__(self):
        self._model = None
        self._lock = asyncio.Lock()
    
    async def get_model(self):
        """Lazy load embedding model."""
        if self._model is None:
            async with self._lock:
                if self._model is None:
                    logger.info(f"Loading embedding model: {settings.EMBEDDING_MODEL}")
                    from sentence_transformers import SentenceTransformer
                    self._model = SentenceTransformer(settings.EMBEDDING_MODEL)
                    logger.info("✅ Embedding model loaded")
        return self._model
    
    async def encode(self, text: str) -> List[float]:
        """Generate embedding for text."""
        model = await self.get_model()
        # Run in thread pool to avoid blocking
        loop = asyncio.get_event_loop()
        embedding = await loop.run_in_executor(
            None,
            lambda: model.encode(text, normalize_embeddings=True)
        )
        return embedding.tolist()

embedding_service = EmbeddingService()

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("💾 Memory Service starting up...")
    
    # Initialize database pool
    try:
        app.state.db_pool = await asyncpg.create_pool(
            settings.DATABASE_URL,
            min_size=2,
            max_size=10
        )
        logger.info("✅ Database pool created")
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
    
    yield
    
    logger.info("🛑 Memory Service shutting down...")
    if app.state.db_pool:
        await app.state.db_pool.close()
    if app.state.redis:
        await app.state.redis.close()

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - Memory Service",
    description="Knowledge Engine - Structured and Semantic Memory",
    version="1.0.0",
    lifespan=lifespan
)

# =============================================================================
# Health Endpoints
# =============================================================================

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    db_status = "healthy"
    redis_status = "healthy"
    
    try:
        if app.state.db_pool:
            async with app.state.db_pool.acquire() as conn:
                await conn.fetchval("SELECT 1")
        else:
            db_status = "not configured"
    except Exception:
        db_status = "unhealthy"
    
    try:
        if app.state.redis:
            await app.state.redis.ping()
        else:
            redis_status = "not configured"
    except Exception:
        redis_status = "unhealthy"
    
    return HealthResponse(
        status="healthy" if db_status == "healthy" else "degraded",
        database=db_status,
        redis=redis_status,
        embedding_model=settings.EMBEDDING_MODEL,
        timestamp=datetime.utcnow().isoformat()
    )

# =============================================================================
# Conversation Storage
# =============================================================================

@app.post("/store")
async def store_messages(request: StoreRequest):
    """Store conversation messages."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    async with app.state.db_pool.acquire() as conn:
        async with conn.transaction():
            # Ensure conversation exists
            conversation = await conn.fetchrow(
                """
                INSERT INTO conversations (id, user_id, title, started_at, last_message_at)
                VALUES ($1::uuid, $2, $3, NOW(), NOW())
                ON CONFLICT (id) DO UPDATE SET 
                    last_message_at = NOW(),
                    message_count = conversations.message_count + $4
                RETURNING id
                """,
                request.conversation_id,
                request.user_id,
                request.messages[0].content[:50] if request.messages else "New Chat",
                len(request.messages)
            )
            
            # Store each message
            for msg in request.messages:
                msg_id = await conn.fetchval(
                    """
                    INSERT INTO messages (conversation_id, role, content, metadata, created_at)
                    VALUES ($1::uuid, $2, $3, $4, NOW())
                    RETURNING id
                    """,
                    request.conversation_id,
                    msg.role,
                    msg.content,
                    msg.metadata or {}
                )
                
                # Generate and store embedding for semantic search
                try:
                    embedding = await embedding_service.encode(msg.content)
                    await conn.execute(
                        """
                        INSERT INTO memory_embeddings 
                        (user_id, source_type, source_id, content, embedding, created_at)
                        VALUES ($1, 'conversation', $2, $3, $4, NOW())
                        """,
                        request.user_id,
                        msg_id,
                        msg.content[:500],  # Truncate for storage
                        embedding
                    )
                except Exception as e:
                    logger.warning(f"Failed to store embedding: {e}")
    
    return {"status": "stored", "conversation_id": request.conversation_id}

@app.get("/history/{conversation_id}")
async def get_history(conversation_id: str, limit: int = 20, offset: int = 0):
    """Get conversation history."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    async with app.state.db_pool.acquire() as conn:
        messages = await conn.fetch(
            """
            SELECT id, role, content, created_at, metadata
            FROM messages
            WHERE conversation_id = $1::uuid
            ORDER BY created_at ASC
            LIMIT $2 OFFSET $3
            """,
            conversation_id,
            limit,
            offset
        )
        
        return {
            "conversation_id": conversation_id,
            "messages": [
                {
                    "id": str(m["id"]),
                    "role": m["role"],
                    "content": m["content"],
                    "timestamp": m["created_at"].isoformat(),
                    "metadata": m["metadata"]
                }
                for m in messages
            ]
        }

@app.get("/conversations/{user_id}")
async def list_conversations(user_id: str, limit: int = 20):
    """List conversations for a user."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    async with app.state.db_pool.acquire() as conn:
        conversations = await conn.fetch(
            """
            SELECT id, title, summary, started_at, last_message_at, message_count
            FROM conversations
            WHERE user_id = $1
            ORDER BY last_message_at DESC
            LIMIT $2
            """,
            user_id,
            limit
        )
        
        return {
            "user_id": user_id,
            "conversations": [
                {
                    "id": str(c["id"]),
                    "title": c["title"],
                    "summary": c["summary"],
                    "started_at": c["started_at"].isoformat(),
                    "last_message_at": c["last_message_at"].isoformat(),
                    "message_count": c["message_count"]
                }
                for c in conversations
            ]
        }

# =============================================================================
# Semantic Search
# =============================================================================

@app.post("/search")
async def semantic_search(request: SearchRequest):
    """Search memory using semantic similarity."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    # Generate query embedding
    try:
        query_embedding = await embedding_service.encode(request.query)
    except Exception as e:
        logger.error(f"Embedding generation failed: {e}")
        raise HTTPException(status_code=500, detail="Embedding generation failed")
    
    async with app.state.db_pool.acquire() as conn:
        # Use pgvector for similarity search
        results = await conn.fetch(
            """
            SELECT 
                id,
                content,
                source_type,
                1 - (embedding <=> $1::vector) as similarity,
                metadata
            FROM memory_embeddings
            WHERE user_id = $2
              AND 1 - (embedding <=> $1::vector) > $3
            ORDER BY embedding <=> $1::vector
            LIMIT $4
            """,
            query_embedding,
            request.user_id,
            request.threshold,
            request.limit
        )
        
        return {
            "query": request.query,
            "results": [
                {
                    "id": str(r["id"]),
                    "content": r["content"],
                    "source_type": r["source_type"],
                    "similarity": float(r["similarity"]),
                    "metadata": r["metadata"]
                }
                for r in results
            ]
        }

# =============================================================================
# Structured Facts
# =============================================================================

@app.post("/facts")
async def store_fact(request: FactRequest):
    """Store a structured fact."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    async with app.state.db_pool.acquire() as conn:
        fact_id = await conn.fetchval(
            """
            INSERT INTO facts (user_id, category, subject, predicate, object, confidence, source)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            RETURNING id
            """,
            request.user_id,
            request.category,
            request.subject,
            request.predicate,
            request.object,
            request.confidence,
            request.source
        )
        
        return {"status": "stored", "fact_id": str(fact_id)}

@app.get("/facts/{user_id}")
async def get_facts(
    user_id: str,
    category: Optional[str] = None,
    subject: Optional[str] = None,
    limit: int = 50
):
    """Retrieve facts for a user."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    query = "SELECT * FROM facts WHERE user_id = $1"
    params = [user_id]
    
    if category:
        params.append(category)
        query += f" AND category = ${len(params)}"
    
    if subject:
        params.append(f"%{subject}%")
        query += f" AND subject ILIKE ${len(params)}"
    
    query += f" ORDER BY updated_at DESC LIMIT {limit}"
    
    async with app.state.db_pool.acquire() as conn:
        facts = await conn.fetch(query, *params)
        
        return {
            "user_id": user_id,
            "facts": [
                {
                    "id": str(f["id"]),
                    "category": f["category"],
                    "subject": f["subject"],
                    "predicate": f["predicate"],
                    "object": f["object"],
                    "confidence": float(f["confidence"]),
                    "source": f["source"],
                    "updated_at": f["updated_at"].isoformat()
                }
                for f in facts
            ]
        }

# =============================================================================
# User Profile
# =============================================================================

@app.get("/profile/{user_id}")
async def get_profile(user_id: str):
    """Get user profile."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    async with app.state.db_pool.acquire() as conn:
        profile = await conn.fetchrow(
            """
            SELECT * FROM user_profile WHERE user_id = $1
            """,
            user_id
        )
        
        if not profile:
            raise HTTPException(status_code=404, detail="User not found")
        
        return {
            "user_id": profile["user_id"],
            "name": profile["name"],
            "preferences": profile["preferences"],
            "timezone": profile["timezone"],
            "created_at": profile["created_at"].isoformat()
        }

@app.put("/profile/{user_id}")
async def update_profile(user_id: str, updates: dict):
    """Update user profile."""
    if not app.state.db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")
    
    async with app.state.db_pool.acquire() as conn:
        await conn.execute(
            """
            UPDATE user_profile
            SET name = COALESCE($2, name),
                preferences = preferences || $3,
                timezone = COALESCE($4, timezone),
                updated_at = NOW()
            WHERE user_id = $1
            """,
            user_id,
            updates.get("name"),
            updates.get("preferences", {}),
            updates.get("timezone")
        )
        
        return {"status": "updated", "user_id": user_id}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8002)
