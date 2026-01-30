"""
=============================================================================
Personal AI Operating System - TTS (Text-to-Speech) Service
=============================================================================
Layer 3: Core Intelligence - Voice Output Processing

Uses Piper TTS for fast, high-quality speech synthesis.
Lightweight and CPU-optimized.
"""

import os
import io
import logging
from datetime import datetime
from contextlib import asynccontextmanager
import asyncio

from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    DEFAULT_VOICE = os.getenv("TTS_VOICE", "en_US-lessac-medium")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    MODEL_PATH = os.getenv("MODEL_PATH", "/app/models")
    SAMPLE_RATE = 22050

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("tts-service")

# =============================================================================
# Models
# =============================================================================

class SynthesisRequest(BaseModel):
    text: str
    voice: str = None
    speed: float = 1.0

class HealthResponse(BaseModel):
    status: str
    default_voice: str
    timestamp: str

# =============================================================================
# TTS Service
# =============================================================================

class TTSService:
    """Piper TTS wrapper with lazy loading."""
    
    def __init__(self):
        self._voices = {}
        self._lock = asyncio.Lock()
    
    async def get_voice(self, voice_name: str = None):
        """Get or load a voice model."""
        voice = voice_name or settings.DEFAULT_VOICE
        
        if voice not in self._voices:
            async with self._lock:
                if voice not in self._voices:
                    logger.info(f"Loading voice: {voice}")
                    try:
                        from piper import PiperVoice
                        
                        # Try to load from local models first
                        model_path = os.path.join(settings.MODEL_PATH, f"{voice}.onnx")
                        config_path = os.path.join(settings.MODEL_PATH, f"{voice}.onnx.json")
                        
                        if os.path.exists(model_path) and os.path.exists(config_path):
                            self._voices[voice] = PiperVoice.load(model_path, config_path)
                        else:
                            # Download model
                            self._voices[voice] = PiperVoice.load(voice, download_dir=settings.MODEL_PATH)
                        
                        logger.info(f"✅ Voice loaded: {voice}")
                    except Exception as e:
                        logger.error(f"Failed to load voice {voice}: {e}")
                        raise
        
        return self._voices.get(voice)
    
    async def synthesize(self, text: str, voice: str = None, speed: float = 1.0) -> bytes:
        """Synthesize speech from text."""
        voice_model = await self.get_voice(voice)
        
        if voice_model is None:
            raise ValueError(f"Voice not available: {voice}")
        
        # Run synthesis in thread pool
        loop = asyncio.get_event_loop()
        
        def _synthesize():
            import soundfile as sf
            import numpy as np
            
            audio_data = []
            for audio_bytes in voice_model.synthesize_stream_raw(text):
                audio_data.append(np.frombuffer(audio_bytes, dtype=np.int16))
            
            if not audio_data:
                return b''
            
            audio = np.concatenate(audio_data)
            
            # Adjust speed if needed
            if speed != 1.0:
                # Simple resampling for speed adjustment
                indices = np.round(np.arange(0, len(audio), speed)).astype(int)
                indices = indices[indices < len(audio)]
                audio = audio[indices]
            
            # Convert to WAV bytes
            buffer = io.BytesIO()
            sf.write(buffer, audio, settings.SAMPLE_RATE, format='WAV')
            buffer.seek(0)
            return buffer.read()
        
        return await loop.run_in_executor(None, _synthesize)

tts_service = TTSService()

# =============================================================================
# Fallback TTS (if Piper fails)
# =============================================================================

async def fallback_synthesize(text: str) -> bytes:
    """Fallback TTS using espeak-ng."""
    import subprocess
    import tempfile
    
    with tempfile.NamedTemporaryFile(suffix='.wav', delete=False) as tmp:
        tmp_path = tmp.name
    
    try:
        subprocess.run(
            ['espeak-ng', '-w', tmp_path, text],
            check=True,
            capture_output=True
        )
        
        with open(tmp_path, 'rb') as f:
            return f.read()
    finally:
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("🔊 TTS Service starting up...")
    yield
    logger.info("🛑 TTS Service shutting down...")

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - TTS Service",
    description="Text-to-Speech Synthesis",
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
        default_voice=settings.DEFAULT_VOICE,
        timestamp=datetime.utcnow().isoformat()
    )

@app.post("/synthesize")
async def synthesize(request: SynthesisRequest):
    """
    Synthesize speech from text.
    
    Returns WAV audio stream.
    """
    if not request.text.strip():
        raise HTTPException(status_code=400, detail="Text cannot be empty")
    
    if len(request.text) > 5000:
        raise HTTPException(status_code=400, detail="Text too long (max 5000 chars)")
    
    logger.info(f"TTS request: {request.text[:50]}...")
    
    try:
        audio_bytes = await tts_service.synthesize(
            request.text,
            voice=request.voice,
            speed=request.speed
        )
    except Exception as e:
        logger.warning(f"Piper TTS failed, using fallback: {e}")
        try:
            audio_bytes = await fallback_synthesize(request.text)
        except Exception as e2:
            logger.error(f"Fallback TTS also failed: {e2}")
            raise HTTPException(status_code=500, detail="Speech synthesis failed")
    
    return StreamingResponse(
        io.BytesIO(audio_bytes),
        media_type="audio/wav",
        headers={"Content-Disposition": "attachment; filename=speech.wav"}
    )

@app.get("/voices")
async def list_voices():
    """List available voices."""
    return {
        "default": settings.DEFAULT_VOICE,
        "available": [
            {"id": "en_US-lessac-medium", "language": "English (US)", "quality": "medium"},
            {"id": "en_GB-alba-medium", "language": "English (UK)", "quality": "medium"},
            {"id": "de_DE-thorsten-medium", "language": "German", "quality": "medium"},
            {"id": "es_ES-carlfm-x-low", "language": "Spanish", "quality": "low"},
            {"id": "fr_FR-upmc-medium", "language": "French", "quality": "medium"}
        ],
        "note": "Additional voices can be added to the models directory"
    }

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8004)
