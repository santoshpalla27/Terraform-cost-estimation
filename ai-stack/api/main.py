"""
=============================================================================
Personal AI Operating System - API Gateway
=============================================================================
Layer 2: Gateway Layer - Request orchestration and session management

This is the main entry point for all client interactions.
Routes requests to appropriate backend services.
"""

import os
import uuid
import time
import logging
from datetime import datetime
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, HTTPException, Request, UploadFile, File, WebSocket, Depends, Header
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel, Field
from starlette.middleware.base import BaseHTTPMiddleware
import httpx
import redis.asyncio as redis

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    BRAIN_URL = os.getenv("BRAIN_URL", "http://ai-brain:8001")
    MEMORY_URL = os.getenv("MEMORY_URL", "http://memory-service:8002")
    STT_URL = os.getenv("STT_URL", "http://stt-service:8003")
    TTS_URL = os.getenv("TTS_URL", "http://tts-service:8004")
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    
    # Security settings
    AUTH_ENABLED = os.getenv("AUTH_ENABLED", "false").lower() == "true"
    API_KEYS = set(k.strip() for k in os.getenv("API_KEYS", "").split(",") if k.strip())
    RATE_LIMIT_RPM = int(os.getenv("RATE_LIMIT_RPM", "60"))
    CORS_ORIGINS = [o.strip() for o in os.getenv("CORS_ORIGINS", "*").split(",")]

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("api-gateway")

# =============================================================================
# Security Utilities
# =============================================================================

class RateLimiter:
    """In-memory sliding window rate limiter."""
    
    def __init__(self, requests_per_minute: int):
        self.rpm = requests_per_minute
        self.window = 60
        self._requests: dict = {}
    
    def is_allowed(self, client_id: str) -> bool:
        now = time.time()
        window_start = now - self.window
        
        if client_id not in self._requests:
            self._requests[client_id] = []
        
        # Clean old entries
        self._requests[client_id] = [ts for ts in self._requests[client_id] if ts > window_start]
        
        if len(self._requests[client_id]) >= self.rpm:
            return False
        
        self._requests[client_id].append(now)
        return True
    
    def get_remaining(self, client_id: str) -> int:
        now = time.time()
        window_start = now - self.window
        
        if client_id not in self._requests:
            return self.rpm
        
        current = len([ts for ts in self._requests[client_id] if ts > window_start])
        return max(0, self.rpm - current)

class CircuitBreaker:
    """Circuit breaker for external service calls."""
    
    def __init__(self, name: str, threshold: int = 5, timeout: int = 30):
        self.name = name
        self.threshold = threshold
        self.timeout = timeout
        self.failures = 0
        self.last_failure: Optional[float] = None
        self.state = "closed"  # closed, open, half_open
    
    def can_execute(self) -> bool:
        if self.state == "closed":
            return True
        if self.state == "open":
            if self.last_failure and time.time() - self.last_failure >= self.timeout:
                self.state = "half_open"
                return True
            return False
        return True  # half_open
    
    def record_success(self):
        self.failures = 0
        self.state = "closed"
    
    def record_failure(self):
        self.failures += 1
        self.last_failure = time.time()
        if self.failures >= self.threshold:
            self.state = "open"
            logger.warning(f"Circuit breaker '{self.name}' opened")

# Global instances
rate_limiter = RateLimiter(settings.RATE_LIMIT_RPM)
brain_circuit = CircuitBreaker("brain", threshold=3, timeout=20)

# =============================================================================
# Auth Middleware
# =============================================================================

class SecurityMiddleware(BaseHTTPMiddleware):
    """Authentication and rate limiting middleware."""
    
    EXCLUDED_PATHS = {"/health", "/api/", "/docs", "/openapi.json", "/redoc"}
    
    async def dispatch(self, request: Request, call_next):
        path = request.url.path
        
        # Skip middleware for excluded paths
        if any(path == p or path.startswith(p) for p in self.EXCLUDED_PATHS if p.endswith("/")):
            pass
        elif path in self.EXCLUDED_PATHS:
            pass
        else:
            # Rate limiting
            client_ip = request.client.host if request.client else "unknown"
            if not rate_limiter.is_allowed(client_ip):
                return JSONResponse(
                    status_code=429,
                    content={"detail": "Rate limit exceeded"},
                    headers={"X-RateLimit-Remaining": "0"}
                )
            
            # Auth check
            if settings.AUTH_ENABLED:
                auth_header = request.headers.get("Authorization", "")
                api_key = auth_header.replace("Bearer ", "") if auth_header.startswith("Bearer ") else auth_header
                
                if not api_key or api_key not in settings.API_KEYS:
                    return JSONResponse(
                        status_code=401,
                        content={"detail": "Invalid or missing API key"}
                    )
        
        response = await call_next(request)
        
        # Add rate limit headers
        client_ip = request.client.host if request.client else "unknown"
        response.headers["X-RateLimit-Remaining"] = str(rate_limiter.get_remaining(client_ip))
        
        return response

# =============================================================================
# Models
# =============================================================================

class ChatRequest(BaseModel):
    message: str = Field(..., min_length=1, max_length=10000)
    conversation_id: Optional[str] = None
    user_id: str = "primary"
    stream: bool = False

class ChatResponse(BaseModel):
    message: str
    conversation_id: str
    timestamp: str
    tokens_used: Optional[int] = None
    agent_triggered: Optional[str] = None

class HealthResponse(BaseModel):
    status: str
    timestamp: str
    services: dict

class VoiceRequest(BaseModel):
    conversation_id: Optional[str] = None
    user_id: str = "primary"
    return_audio: bool = True

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle - startup and shutdown."""
    # Startup
    logger.info("🚀 API Gateway starting up...")
    
    # Initialize Redis connection
    try:
        app.state.redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        await app.state.redis.ping()
        logger.info("✅ Redis connection established")
    except Exception as e:
        logger.warning(f"⚠️ Redis connection failed: {e}")
        app.state.redis = None
    
    # Initialize HTTP client
    app.state.http_client = httpx.AsyncClient(timeout=120.0)
    logger.info("✅ HTTP client initialized")
    
    yield
    
    # Shutdown
    logger.info("🛑 API Gateway shutting down...")
    await app.state.http_client.aclose()
    if app.state.redis:
        await app.state.redis.close()

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - API Gateway",
    description="Gateway layer for Personal AI Operating System",
    version="1.0.0",
    lifespan=lifespan
)

# Security middleware (auth + rate limiting)
app.add_middleware(SecurityMiddleware)

# CORS middleware - use configured origins
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.CORS_ORIGINS if settings.CORS_ORIGINS != ["*"] else ["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# =============================================================================
# Health Endpoints
# =============================================================================

@app.get("/health", response_model=HealthResponse)
async def health_check(request: Request):
    """Health check endpoint for load balancers and monitoring."""
    services_status = {}
    
    # Check Brain service
    try:
        resp = await request.app.state.http_client.get(f"{settings.BRAIN_URL}/health")
        services_status["brain"] = "healthy" if resp.status_code == 200 else "unhealthy"
    except Exception:
        services_status["brain"] = "unreachable"
    
    # Check Memory service
    try:
        resp = await request.app.state.http_client.get(f"{settings.MEMORY_URL}/health")
        services_status["memory"] = "healthy" if resp.status_code == 200 else "unhealthy"
    except Exception:
        services_status["memory"] = "unreachable"
    
    # Check Redis
    try:
        if request.app.state.redis:
            await request.app.state.redis.ping()
            services_status["redis"] = "healthy"
        else:
            services_status["redis"] = "not configured"
    except Exception:
        services_status["redis"] = "unreachable"
    
    overall_status = "healthy" if all(
        s in ["healthy", "not configured"] for s in services_status.values()
    ) else "degraded"
    
    return HealthResponse(
        status=overall_status,
        timestamp=datetime.utcnow().isoformat(),
        services=services_status
    )

@app.get("/api/")
async def api_root():
    """API root information."""
    return {
        "name": "Personal AI OS",
        "version": "1.0.0",
        "endpoints": {
            "chat": "/api/chat",
            "voice": "/api/voice/transcribe",
            "health": "/health"
        }
    }

# =============================================================================
# Chat Endpoints
# =============================================================================

@app.post("/api/chat", response_model=ChatResponse)
async def chat(request: Request, chat_request: ChatRequest):
    """
    Main chat endpoint.
    
    Sends user message to AI Brain service and returns response.
    Optionally streams response via WebSocket.
    """
    request_id = str(uuid.uuid4())
    logger.info(f"[{request_id}] Chat request: {chat_request.message[:50]}...")
    
    # Generate conversation ID if not provided
    conversation_id = chat_request.conversation_id or str(uuid.uuid4())
    
    try:
        # Forward to Brain service
        brain_response = await request.app.state.http_client.post(
            f"{settings.BRAIN_URL}/chat",
            json={
                "message": chat_request.message,
                "conversation_id": conversation_id,
                "user_id": chat_request.user_id
            }
        )
        
        if brain_response.status_code != 200:
            logger.error(f"[{request_id}] Brain service error: {brain_response.status_code}")
            raise HTTPException(
                status_code=502,
                detail="AI Brain service unavailable"
            )
        
        result = brain_response.json()
        
        # Log to Redis for analytics (fire and forget)
        if request.app.state.redis:
            try:
                await request.app.state.redis.lpush(
                    "chat_log",
                    f"{datetime.utcnow().isoformat()}|{conversation_id}|{chat_request.message[:100]}"
                )
            except Exception as e:
                logger.warning(f"[{request_id}] Redis logging failed: {e}")
        
        return ChatResponse(
            message=result.get("response", ""),
            conversation_id=conversation_id,
            timestamp=datetime.utcnow().isoformat(),
            tokens_used=result.get("tokens_used"),
            agent_triggered=result.get("agent_triggered")
        )
        
    except httpx.RequestError as e:
        logger.error(f"[{request_id}] Request to Brain failed: {e}")
        raise HTTPException(
            status_code=503,
            detail="Service temporarily unavailable"
        )

# =============================================================================
# Voice Endpoints
# =============================================================================

@app.post("/api/voice/transcribe")
async def transcribe_audio(
    request: Request,
    audio: UploadFile = File(...),
    return_text_only: bool = True
):
    """
    Transcribe audio to text using STT service.
    """
    request_id = str(uuid.uuid4())
    logger.info(f"[{request_id}] Voice transcription request: {audio.filename}")
    
    try:
        # Read audio file
        audio_content = await audio.read()
        
        # Forward to STT service
        files = {"audio": (audio.filename, audio_content, audio.content_type)}
        stt_response = await request.app.state.http_client.post(
            f"{settings.STT_URL}/transcribe",
            files=files
        )
        
        if stt_response.status_code != 200:
            raise HTTPException(status_code=502, detail="STT service error")
        
        result = stt_response.json()
        
        if return_text_only:
            return {"text": result.get("text", "")}
        
        return result
        
    except httpx.RequestError as e:
        logger.error(f"[{request_id}] STT request failed: {e}")
        raise HTTPException(status_code=503, detail="Voice service unavailable")

@app.post("/api/voice/synthesize")
async def synthesize_speech(
    request: Request,
    text: str,
    voice: str = "default"
):
    """
    Synthesize speech from text using TTS service.
    Returns audio stream.
    """
    request_id = str(uuid.uuid4())
    logger.info(f"[{request_id}] TTS request: {text[:50]}...")
    
    try:
        tts_response = await request.app.state.http_client.post(
            f"{settings.TTS_URL}/synthesize",
            json={"text": text, "voice": voice}
        )
        
        if tts_response.status_code != 200:
            raise HTTPException(status_code=502, detail="TTS service error")
        
        return StreamingResponse(
            iter([tts_response.content]),
            media_type="audio/wav"
        )
        
    except httpx.RequestError as e:
        logger.error(f"[{request_id}] TTS request failed: {e}")
        raise HTTPException(status_code=503, detail="Voice service unavailable")

@app.post("/api/voice/chat")
async def voice_chat(
    request: Request,
    audio: UploadFile = File(...),
    conversation_id: Optional[str] = None,
    user_id: str = "primary"
):
    """
    Full voice chat: STT → Brain → TTS
    
    Accepts audio input, returns audio response.
    """
    request_id = str(uuid.uuid4())
    logger.info(f"[{request_id}] Voice chat request")
    
    try:
        # Step 1: Transcribe audio
        audio_content = await audio.read()
        files = {"audio": (audio.filename, audio_content, audio.content_type)}
        
        stt_response = await request.app.state.http_client.post(
            f"{settings.STT_URL}/transcribe",
            files=files
        )
        
        if stt_response.status_code != 200:
            raise HTTPException(status_code=502, detail="STT failed")
        
        transcribed_text = stt_response.json().get("text", "")
        logger.info(f"[{request_id}] Transcribed: {transcribed_text[:100]}")
        
        # Step 2: Process with Brain
        brain_response = await request.app.state.http_client.post(
            f"{settings.BRAIN_URL}/chat",
            json={
                "message": transcribed_text,
                "conversation_id": conversation_id or str(uuid.uuid4()),
                "user_id": user_id
            }
        )
        
        if brain_response.status_code != 200:
            raise HTTPException(status_code=502, detail="Brain failed")
        
        ai_response = brain_response.json().get("response", "")
        
        # Step 3: Synthesize speech
        tts_response = await request.app.state.http_client.post(
            f"{settings.TTS_URL}/synthesize",
            json={"text": ai_response}
        )
        
        if tts_response.status_code != 200:
            # Fallback to text response
            return JSONResponse({
                "transcribed": transcribed_text,
                "response": ai_response,
                "audio": None
            })
        
        return StreamingResponse(
            iter([tts_response.content]),
            media_type="audio/wav",
            headers={
                "X-Transcribed-Text": transcribed_text[:200],
                "X-Response-Text": ai_response[:200]
            }
        )
        
    except httpx.RequestError as e:
        logger.error(f"[{request_id}] Voice chat failed: {e}")
        raise HTTPException(status_code=503, detail="Voice service unavailable")

# =============================================================================
# WebSocket for Streaming
# =============================================================================

@app.websocket("/ws/chat")
async def websocket_chat(websocket: WebSocket):
    """
    WebSocket endpoint for streaming chat responses.
    """
    await websocket.accept()
    logger.info("WebSocket connection established")
    
    try:
        while True:
            # Receive message
            data = await websocket.receive_json()
            message = data.get("message", "")
            conversation_id = data.get("conversation_id", str(uuid.uuid4()))
            
            # Forward to Brain with streaming
            async with httpx.AsyncClient() as client:
                async with client.stream(
                    "POST",
                    f"{settings.BRAIN_URL}/chat/stream",
                    json={
                        "message": message,
                        "conversation_id": conversation_id
                    }
                ) as response:
                    async for chunk in response.aiter_text():
                        await websocket.send_json({
                            "type": "chunk",
                            "content": chunk
                        })
            
            await websocket.send_json({
                "type": "complete",
                "conversation_id": conversation_id
            })
            
    except Exception as e:
        logger.error(f"WebSocket error: {e}")
    finally:
        logger.info("WebSocket connection closed")

# =============================================================================
# Session Management
# =============================================================================

@app.get("/api/sessions/{user_id}")
async def list_sessions(request: Request, user_id: str):
    """List conversation sessions for a user."""
    try:
        response = await request.app.state.http_client.get(
            f"{settings.MEMORY_URL}/conversations/{user_id}"
        )
        return response.json()
    except Exception as e:
        logger.error(f"Failed to list sessions: {e}")
        raise HTTPException(status_code=503, detail="Memory service unavailable")

@app.get("/api/sessions/{user_id}/{conversation_id}")
async def get_session(request: Request, user_id: str, conversation_id: str):
    """Get a specific conversation session."""
    try:
        response = await request.app.state.http_client.get(
            f"{settings.MEMORY_URL}/conversations/{user_id}/{conversation_id}"
        )
        return response.json()
    except Exception as e:
        logger.error(f"Failed to get session: {e}")
        raise HTTPException(status_code=503, detail="Memory service unavailable")

# =============================================================================
# Error Handlers
# =============================================================================

@app.exception_handler(Exception)
async def global_exception_handler(request: Request, exc: Exception):
    logger.error(f"Unhandled exception: {exc}", exc_info=True)
    return JSONResponse(
        status_code=500,
        content={"detail": "Internal server error"}
    )

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
