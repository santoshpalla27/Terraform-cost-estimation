"""
=============================================================================
Personal AI Operating System - Shared Utilities
=============================================================================
Common utilities for resilience, security, and cross-service concerns.
"""

import os
import time
import logging
import asyncio
import functools
from typing import Optional, Callable, Any
from datetime import datetime, timedelta

logger = logging.getLogger("ai-os-utils")

# =============================================================================
# Circuit Breaker
# =============================================================================

class CircuitBreaker:
    """
    Circuit breaker pattern for resilient external calls.
    
    States:
    - CLOSED: Normal operation, requests pass through
    - OPEN: Failure threshold exceeded, requests fail immediately
    - HALF_OPEN: Testing if service recovered, limited requests
    """
    
    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"
    
    def __init__(
        self,
        name: str,
        failure_threshold: int = 5,
        recovery_timeout: int = 30,
        half_open_requests: int = 3
    ):
        self.name = name
        self.failure_threshold = failure_threshold
        self.recovery_timeout = recovery_timeout
        self.half_open_requests = half_open_requests
        
        self.state = self.CLOSED
        self.failures = 0
        self.successes = 0
        self.last_failure_time: Optional[float] = None
        self.half_open_count = 0
    
    def record_success(self):
        """Record a successful call."""
        if self.state == self.HALF_OPEN:
            self.successes += 1
            if self.successes >= self.half_open_requests:
                self._close()
        elif self.state == self.CLOSED:
            self.failures = 0
    
    def record_failure(self):
        """Record a failed call."""
        self.failures += 1
        self.last_failure_time = time.time()
        
        if self.state == self.HALF_OPEN:
            self._open()
        elif self.failures >= self.failure_threshold:
            self._open()
    
    def can_execute(self) -> bool:
        """Check if a call should be attempted."""
        if self.state == self.CLOSED:
            return True
        
        if self.state == self.OPEN:
            if self._should_attempt_recovery():
                self._half_open()
                return True
            return False
        
        # HALF_OPEN: allow limited requests
        if self.half_open_count < self.half_open_requests:
            self.half_open_count += 1
            return True
        return False
    
    def _should_attempt_recovery(self) -> bool:
        if self.last_failure_time is None:
            return True
        return time.time() - self.last_failure_time >= self.recovery_timeout
    
    def _open(self):
        self.state = self.OPEN
        logger.warning(f"Circuit breaker '{self.name}' OPENED")
    
    def _close(self):
        self.state = self.CLOSED
        self.failures = 0
        self.successes = 0
        self.half_open_count = 0
        logger.info(f"Circuit breaker '{self.name}' CLOSED")
    
    def _half_open(self):
        self.state = self.HALF_OPEN
        self.half_open_count = 0
        self.successes = 0
        logger.info(f"Circuit breaker '{self.name}' HALF_OPEN")

# =============================================================================
# Retry Logic
# =============================================================================

async def retry_async(
    func: Callable,
    max_retries: int = 3,
    base_delay: float = 0.5,
    max_delay: float = 10.0,
    exponential: bool = True,
    exceptions: tuple = (Exception,)
) -> Any:
    """
    Retry an async function with exponential backoff.
    """
    last_exception = None
    
    for attempt in range(max_retries + 1):
        try:
            return await func()
        except exceptions as e:
            last_exception = e
            if attempt < max_retries:
                if exponential:
                    delay = min(base_delay * (2 ** attempt), max_delay)
                else:
                    delay = base_delay
                logger.warning(
                    f"Retry {attempt + 1}/{max_retries} after {delay:.1f}s: {e}"
                )
                await asyncio.sleep(delay)
    
    raise last_exception

def with_retry(max_retries: int = 3, base_delay: float = 0.5):
    """Decorator for adding retry logic to async functions."""
    def decorator(func):
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            return await retry_async(
                lambda: func(*args, **kwargs),
                max_retries=max_retries,
                base_delay=base_delay
            )
        return wrapper
    return decorator

# =============================================================================
# Redis Resilience Wrapper
# =============================================================================

class ResilientRedis:
    """
    Redis wrapper with circuit breaker and retry logic.
    
    Falls back gracefully when Redis is unavailable.
    """
    
    def __init__(self, redis_client, fallback_enabled: bool = True):
        self.redis = redis_client
        self.fallback_enabled = fallback_enabled
        self.circuit = CircuitBreaker("redis", failure_threshold=3, recovery_timeout=15)
        self._connected = redis_client is not None
    
    @property
    def available(self) -> bool:
        return self._connected and self.circuit.can_execute()
    
    async def get(self, key: str, default: Any = None) -> Any:
        """Get with fallback."""
        if not self.available:
            return default
        try:
            if self.circuit.can_execute():
                result = await self.redis.get(key)
                self.circuit.record_success()
                return result if result is not None else default
        except Exception as e:
            self.circuit.record_failure()
            logger.warning(f"Redis GET failed: {e}")
        return default
    
    async def set(self, key: str, value: Any, ex: int = None) -> bool:
        """Set with fallback."""
        if not self.available:
            return False
        try:
            if self.circuit.can_execute():
                if ex:
                    await self.redis.setex(key, ex, value)
                else:
                    await self.redis.set(key, value)
                self.circuit.record_success()
                return True
        except Exception as e:
            self.circuit.record_failure()
            logger.warning(f"Redis SET failed: {e}")
        return False
    
    async def lpush(self, key: str, *values) -> bool:
        """List push with fallback."""
        if not self.available:
            return False
        try:
            if self.circuit.can_execute():
                await self.redis.lpush(key, *values)
                self.circuit.record_success()
                return True
        except Exception as e:
            self.circuit.record_failure()
            logger.warning(f"Redis LPUSH failed: {e}")
        return False
    
    async def ping(self) -> bool:
        """Check Redis connectivity."""
        if not self._connected:
            return False
        try:
            await self.redis.ping()
            self.circuit.record_success()
            return True
        except Exception as e:
            self.circuit.record_failure()
            return False

# =============================================================================
# API Key Authentication
# =============================================================================

class APIKeyAuth:
    """
    Simple API key authentication.
    
    Keys loaded from environment or config.
    """
    
    def __init__(self):
        # Load API keys from environment
        keys_str = os.getenv("API_KEYS", "")
        self.enabled = os.getenv("AUTH_ENABLED", "false").lower() == "true"
        
        if keys_str:
            self.valid_keys = set(k.strip() for k in keys_str.split(",") if k.strip())
        else:
            # Generate a default key for development
            self.valid_keys = set()
    
    def validate(self, api_key: Optional[str]) -> bool:
        """Validate an API key."""
        if not self.enabled:
            return True
        if not api_key:
            return False
        return api_key in self.valid_keys
    
    def get_key_from_header(self, authorization: Optional[str]) -> Optional[str]:
        """Extract API key from Authorization header."""
        if not authorization:
            return None
        if authorization.startswith("Bearer "):
            return authorization[7:]
        return authorization

# =============================================================================
# Rate Limiter
# =============================================================================

class RateLimiter:
    """
    Simple in-memory rate limiter.
    
    Uses sliding window algorithm.
    """
    
    def __init__(self, requests_per_minute: int = 60):
        self.requests_per_minute = requests_per_minute
        self.window_seconds = 60
        self._requests: dict = {}  # key -> list of timestamps
    
    def is_allowed(self, key: str) -> bool:
        """Check if request is allowed for this key."""
        now = time.time()
        window_start = now - self.window_seconds
        
        if key not in self._requests:
            self._requests[key] = []
        
        # Clean old entries
        self._requests[key] = [
            ts for ts in self._requests[key] if ts > window_start
        ]
        
        if len(self._requests[key]) >= self.requests_per_minute:
            return False
        
        self._requests[key].append(now)
        return True
    
    def get_remaining(self, key: str) -> int:
        """Get remaining requests for this key."""
        now = time.time()
        window_start = now - self.window_seconds
        
        if key not in self._requests:
            return self.requests_per_minute
        
        current = len([ts for ts in self._requests[key] if ts > window_start])
        return max(0, self.requests_per_minute - current)

# =============================================================================
# Singleton instances
# =============================================================================

_auth = None
_rate_limiter = None

def get_auth() -> APIKeyAuth:
    global _auth
    if _auth is None:
        _auth = APIKeyAuth()
    return _auth

def get_rate_limiter(requests_per_minute: int = 60) -> RateLimiter:
    global _rate_limiter
    if _rate_limiter is None:
        _rate_limiter = RateLimiter(requests_per_minute)
    return _rate_limiter
