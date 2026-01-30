"""
=============================================================================
Personal AI Operating System - STT (Speech-to-Text) Service
=============================================================================
Layer 3: Core Intelligence - Voice Input Processing

Uses OpenAI Whisper for multilingual speech recognition.
Supports tiny/base/small models based on RAM constraints.
"""

import os
import logging
import tempfile
from datetime import datetime
from contextlib import asynccontextmanager
import asyncio

from fastapi import FastAPI, HTTPException, UploadFile, File
from pydantic import BaseModel

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    WHISPER_MODEL = os.getenv("WHISPER_MODEL", "tiny")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    MODEL_PATH = os.getenv("MODEL_PATH", "/app/models")

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("stt-service")

# =============================================================================
# Models
# =============================================================================

class TranscriptionResponse(BaseModel):
    text: str
    language: str
    duration: float
    model: str

class HealthResponse(BaseModel):
    status: str
    model: str
    model_loaded: bool
    timestamp: str

# =============================================================================
# Whisper Service (Lazy Loading)
# =============================================================================

class WhisperService:
    def __init__(self):
        self._model = None
        self._lock = asyncio.Lock()
    
    async def get_model(self):
        """Lazy load Whisper model."""
        if self._model is None:
            async with self._lock:
                if self._model is None:
                    logger.info(f"Loading Whisper model: {settings.WHISPER_MODEL}")
                    import whisper
                    
                    # Download to persistent directory
                    self._model = whisper.load_model(
                        settings.WHISPER_MODEL,
                        download_root=settings.MODEL_PATH
                    )
                    logger.info("✅ Whisper model loaded")
        return self._model
    
    async def transcribe(self, audio_path: str) -> dict:
        """Transcribe audio file."""
        model = await self.get_model()
        
        # Run in thread pool to avoid blocking
        loop = asyncio.get_event_loop()
        result = await loop.run_in_executor(
            None,
            lambda: model.transcribe(audio_path)
        )
        
        return result
    
    @property
    def is_loaded(self) -> bool:
        return self._model is not None

whisper_service = WhisperService()

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("🎤 STT Service starting up...")
    
    # Optionally preload model
    if os.getenv("PRELOAD_MODEL", "false").lower() == "true":
        await whisper_service.get_model()
    
    yield
    
    logger.info("🛑 STT Service shutting down...")

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - STT Service",
    description="Speech-to-Text using Whisper",
    version="1.0.0",
    lifespan=lifespan
)

# =============================================================================
# Endpoints
# =============================================================================

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    return HealthResponse(
        status="healthy",
        model=settings.WHISPER_MODEL,
        model_loaded=whisper_service.is_loaded,
        timestamp=datetime.utcnow().isoformat()
    )

@app.post("/transcribe", response_model=TranscriptionResponse)
async def transcribe(audio: UploadFile = File(...)):
    """
    Transcribe uploaded audio file.
    
    Supported formats: wav, mp3, m4a, flac, ogg, webm
    """
    logger.info(f"Transcription request: {audio.filename}")
    
    # Validate file type
    allowed_extensions = {'.wav', '.mp3', '.m4a', '.flac', '.ogg', '.webm'}
    file_ext = os.path.splitext(audio.filename or "")[1].lower()
    
    if file_ext and file_ext not in allowed_extensions:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported audio format. Allowed: {allowed_extensions}"
        )
    
    try:
        # Save to temporary file
        with tempfile.NamedTemporaryFile(suffix=file_ext or '.wav', delete=False) as tmp:
            content = await audio.read()
            tmp.write(content)
            tmp_path = tmp.name
        
        try:
            # Transcribe
            import time
            start_time = time.time()
            
            result = await whisper_service.transcribe(tmp_path)
            
            duration = time.time() - start_time
            
            return TranscriptionResponse(
                text=result["text"].strip(),
                language=result.get("language", "unknown"),
                duration=duration,
                model=settings.WHISPER_MODEL
            )
            
        finally:
            # Clean up temp file
            os.unlink(tmp_path)
            
    except Exception as e:
        logger.error(f"Transcription failed: {e}")
        raise HTTPException(status_code=500, detail=f"Transcription failed: {str(e)}")

@app.get("/models")
async def list_models():
    """List available Whisper models."""
    return {
        "available": ["tiny", "base", "small", "medium", "large"],
        "current": settings.WHISPER_MODEL,
        "loaded": whisper_service.is_loaded,
        "recommendations": {
            "tiny": "Fastest, lowest quality, ~1GB RAM",
            "base": "Good balance, ~1GB RAM",
            "small": "Better quality, ~2GB RAM",
            "medium": "High quality, ~5GB RAM",
            "large": "Best quality, ~10GB RAM"
        }
    }

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8003)
