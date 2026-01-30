"""
=============================================================================
Personal AI Operating System - AI Brain Service
=============================================================================
Layer 3: Core Intelligence - Decision Engine and Orchestrator

The Brain is the coordination layer. It:
- Retrieves memory context
- Injects identity/personality
- Constructs prompts
- Calls LLM runtime
- Interprets responses
- Triggers agents when needed

It does NOT:
- Store data directly (delegates to Memory service)
- Execute actions (delegates to Agent Engine)
"""

import os
import logging
from datetime import datetime
from contextlib import asynccontextmanager
from typing import Optional, AsyncGenerator

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field
import httpx
import redis.asyncio as redis
import yaml

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    OLLAMA_URL = os.getenv("OLLAMA_URL", "http://ollama-runtime:11434")
    MEMORY_SERVICE_URL = os.getenv("MEMORY_SERVICE_URL", "http://memory-service:8002")
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    LLM_MODEL = os.getenv("LLM_MODEL", "qwen2.5:3b")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    IDENTITY_PATH = os.getenv("IDENTITY_PATH", "/app/identity/persona.yaml")

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("ai-brain")

# =============================================================================
# Identity System
# =============================================================================

DEFAULT_IDENTITY = {
    "name": "Atlas",
    "role": "Personal AI Assistant",
    "personality": {
        "tone": "friendly, professional, and thoughtful",
        "style": "clear and concise, with a touch of warmth",
        "traits": ["helpful", "curious", "proactive", "honest"]
    },
    "guidelines": [
        "Always be truthful and acknowledge uncertainty",
        "Proactively offer relevant suggestions",
        "Remember context from previous conversations",
        "Respect user privacy and boundaries",
        "Be concise but thorough when needed"
    ],
    "domains": ["general knowledge", "productivity", "technology", "creative tasks"]
}

def load_identity() -> dict:
    """Load identity configuration from YAML file or use defaults."""
    try:
        if os.path.exists(settings.IDENTITY_PATH):
            with open(settings.IDENTITY_PATH, 'r') as f:
                identity = yaml.safe_load(f)
                logger.info(f"Loaded identity: {identity.get('name', 'Unknown')}")
                return identity
    except Exception as e:
        logger.warning(f"Failed to load identity: {e}")
    
    return DEFAULT_IDENTITY

# =============================================================================
# Models
# =============================================================================

class ChatRequest(BaseModel):
    message: str = Field(..., min_length=1, max_length=10000)
    conversation_id: str
    user_id: str = "primary"
    include_memory: bool = True

class ChatResponse(BaseModel):
    response: str
    conversation_id: str
    tokens_used: Optional[int] = None
    agent_triggered: Optional[str] = None
    memory_context_used: bool = False

class HealthResponse(BaseModel):
    status: str
    llm_status: str
    model: str
    timestamp: str

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("🧠 AI Brain starting up...")
    
    # Load identity
    app.state.identity = load_identity()
    logger.info(f"✅ Identity loaded: {app.state.identity.get('name')}")
    
    # Initialize Redis
    try:
        app.state.redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        await app.state.redis.ping()
        logger.info("✅ Redis connected")
    except Exception as e:
        logger.warning(f"⚠️ Redis unavailable: {e}")
        app.state.redis = None
    
    # Initialize HTTP client
    app.state.http_client = httpx.AsyncClient(timeout=120.0)
    
    # Verify LLM is available
    try:
        resp = await app.state.http_client.get(f"{settings.OLLAMA_URL}/api/version")
        logger.info(f"✅ Ollama connected: {resp.json()}")
    except Exception as e:
        logger.warning(f"⚠️ Ollama not ready: {e}")
    
    yield
    
    logger.info("🛑 AI Brain shutting down...")
    await app.state.http_client.aclose()
    if app.state.redis:
        await app.state.redis.close()

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - Brain Service",
    description="Core Intelligence Layer - Decision Engine",
    version="1.0.0",
    lifespan=lifespan
)

# =============================================================================
# Prompt Construction
# =============================================================================

def build_system_prompt(identity: dict) -> str:
    """Build system prompt from identity configuration."""
    name = identity.get("name", "Assistant")
    role = identity.get("role", "AI Assistant")
    personality = identity.get("personality", {})
    guidelines = identity.get("guidelines", [])
    domains = identity.get("domains", [])
    
    tone = personality.get("tone", "helpful")
    style = personality.get("style", "professional")
    traits = personality.get("traits", [])
    
    prompt = f"""You are {name}, a {role}.

PERSONALITY:
- Tone: {tone}
- Style: {style}
- Key traits: {', '.join(traits)}

GUIDELINES:
{chr(10).join(f'- {g}' for g in guidelines)}

EXPERTISE:
{', '.join(domains)}

CURRENT TIME: {datetime.now().strftime('%Y-%m-%d %H:%M %Z')}

Respond naturally as {name}, maintaining consistency with your identity."""
    
    return prompt

async def fetch_memory_context(
    http_client: httpx.AsyncClient,
    user_id: str,
    conversation_id: str,
    query: str
) -> str:
    """Fetch relevant memory context from Memory service."""
    try:
        # Get conversation history
        history_resp = await http_client.get(
            f"{settings.MEMORY_SERVICE_URL}/history/{conversation_id}",
            params={"limit": 10}
        )
        
        history = []
        if history_resp.status_code == 200:
            history = history_resp.json().get("messages", [])
        
        # Get semantic memory (relevant facts)
        semantic_resp = await http_client.post(
            f"{settings.MEMORY_SERVICE_URL}/search",
            json={"query": query, "user_id": user_id, "limit": 5}
        )
        
        facts = []
        if semantic_resp.status_code == 200:
            facts = semantic_resp.json().get("results", [])
        
        # Build context string
        context_parts = []
        
        if history:
            context_parts.append("RECENT CONVERSATION:")
            for msg in history[-5:]:  # Last 5 messages
                role = msg.get("role", "unknown")
                content = msg.get("content", "")[:200]
                context_parts.append(f"  [{role}]: {content}")
        
        if facts:
            context_parts.append("\nRELEVANT MEMORIES:")
            for fact in facts[:3]:
                context_parts.append(f"  - {fact.get('content', '')[:150]}")
        
        return "\n".join(context_parts) if context_parts else ""
        
    except Exception as e:
        logger.warning(f"Failed to fetch memory: {e}")
        return ""

def detect_agent_intent(response: str) -> Optional[str]:
    """
    Detect if the AI response suggests an agent action.
    
    Returns agent type if action is needed, None otherwise.
    """
    # Simple keyword detection - can be made more sophisticated
    triggers = {
        "reminder": ["remind me", "set a reminder", "schedule a reminder"],
        "search": ["let me search", "searching for", "looking up"],
        "calculate": ["calculating", "let me compute"],
        "weather": ["weather forecast", "checking weather"],
        "calendar": ["scheduling", "adding to calendar", "checking schedule"]
    }
    
    response_lower = response.lower()
    for agent, keywords in triggers.items():
        if any(kw in response_lower for kw in keywords):
            return agent
    
    return None

# =============================================================================
# Endpoints
# =============================================================================

@app.get("/health", response_model=HealthResponse)
async def health_check(request: Request):
    """Health check with LLM status."""
    llm_status = "unknown"
    
    try:
        resp = await request.app.state.http_client.get(
            f"{settings.OLLAMA_URL}/api/version"
        )
        llm_status = "healthy" if resp.status_code == 200 else "unhealthy"
    except Exception:
        llm_status = "unreachable"
    
    return HealthResponse(
        status="healthy",
        llm_status=llm_status,
        model=settings.LLM_MODEL,
        timestamp=datetime.utcnow().isoformat()
    )

@app.post("/chat", response_model=ChatResponse)
async def chat(request: Request, chat_request: ChatRequest):
    """
    Main chat endpoint.
    
    1. Fetch memory context
    2. Build prompt with identity
    3. Call LLM
    4. Store response in memory
    5. Detect agent triggers
    6. Return response
    """
    logger.info(f"Chat request: {chat_request.message[:50]}...")
    
    # Fetch memory context
    memory_context = ""
    if chat_request.include_memory:
        memory_context = await fetch_memory_context(
            request.app.state.http_client,
            chat_request.user_id,
            chat_request.conversation_id,
            chat_request.message
        )
    
    # Build system prompt
    system_prompt = build_system_prompt(request.app.state.identity)
    
    # Add memory context to system prompt
    if memory_context:
        system_prompt += f"\n\n{memory_context}"
    
    # Call Ollama
    try:
        ollama_response = await request.app.state.http_client.post(
            f"{settings.OLLAMA_URL}/api/chat",
            json={
                "model": settings.LLM_MODEL,
                "messages": [
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": chat_request.message}
                ],
                "stream": False
            }
        )
        
        if ollama_response.status_code != 200:
            logger.error(f"Ollama error: {ollama_response.status_code}")
            raise HTTPException(status_code=502, detail="LLM service error")
        
        result = ollama_response.json()
        ai_response = result.get("message", {}).get("content", "")
        tokens_used = result.get("eval_count", 0) + result.get("prompt_eval_count", 0)
        
    except httpx.RequestError as e:
        logger.error(f"Ollama request failed: {e}")
        raise HTTPException(status_code=503, detail="LLM service unavailable")
    
    # Store in memory (fire and forget)
    try:
        await request.app.state.http_client.post(
            f"{settings.MEMORY_SERVICE_URL}/store",
            json={
                "conversation_id": chat_request.conversation_id,
                "user_id": chat_request.user_id,
                "messages": [
                    {"role": "user", "content": chat_request.message},
                    {"role": "assistant", "content": ai_response}
                ]
            }
        )
    except Exception as e:
        logger.warning(f"Failed to store in memory: {e}")
    
    # Detect agent triggers
    agent_triggered = detect_agent_intent(ai_response)
    
    return ChatResponse(
        response=ai_response,
        conversation_id=chat_request.conversation_id,
        tokens_used=tokens_used,
        agent_triggered=agent_triggered,
        memory_context_used=bool(memory_context)
    )

@app.post("/chat/stream")
async def chat_stream(request: Request, chat_request: ChatRequest):
    """
    Streaming chat endpoint.
    
    Returns Server-Sent Events for real-time response streaming.
    """
    async def generate() -> AsyncGenerator[str, None]:
        # Build prompt
        system_prompt = build_system_prompt(request.app.state.identity)
        
        if chat_request.include_memory:
            memory_context = await fetch_memory_context(
                request.app.state.http_client,
                chat_request.user_id,
                chat_request.conversation_id,
                chat_request.message
            )
            if memory_context:
                system_prompt += f"\n\n{memory_context}"
        
        # Stream from Ollama
        async with request.app.state.http_client.stream(
            "POST",
            f"{settings.OLLAMA_URL}/api/chat",
            json={
                "model": settings.LLM_MODEL,
                "messages": [
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": chat_request.message}
                ],
                "stream": True
            }
        ) as response:
            async for line in response.aiter_lines():
                if line:
                    import json
                    try:
                        data = json.loads(line)
                        content = data.get("message", {}).get("content", "")
                        if content:
                            yield content
                    except json.JSONDecodeError:
                        continue
    
    return StreamingResponse(
        generate(),
        media_type="text/event-stream"
    )

@app.get("/identity")
async def get_identity(request: Request):
    """Get current AI identity configuration."""
    return request.app.state.identity

@app.put("/identity")
async def update_identity(request: Request, identity: dict):
    """Update AI identity configuration."""
    # Validate required fields
    if "name" not in identity:
        raise HTTPException(status_code=400, detail="Identity must have a name")
    
    request.app.state.identity = identity
    
    # Persist to file
    try:
        os.makedirs(os.path.dirname(settings.IDENTITY_PATH), exist_ok=True)
        with open(settings.IDENTITY_PATH, 'w') as f:
            yaml.safe_dump(identity, f)
    except Exception as e:
        logger.warning(f"Failed to persist identity: {e}")
    
    return {"status": "updated", "identity": identity}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)
