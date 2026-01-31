"""
=============================================================================
Personal AI Operating System - Resilience Module
=============================================================================
Production-grade resilience patterns:
- Circuit breakers for all external calls
- Redis outage detection and alerting
- Retry with exponential backoff
- Graceful degradation
"""

import os
import time
import logging
import asyncio
from typing import Optional, Callable, Any, Dict
from datetime import datetime, timedelta
from functools import wraps

logger = logging.getLogger("resilience")

# =============================================================================
# Circuit Breaker
# =============================================================================

class CircuitBreaker:
    """
    Circuit breaker pattern for external service calls.
    
    States:
    - CLOSED: Normal operation
    - OPEN: Failing, reject calls immediately  
    - HALF_OPEN: Testing recovery
    """
    
    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"
    
    def __init__(
        self,
        name: str,
        failure_threshold: int = 5,
        recovery_timeout: int = 30,
        half_open_max: int = 3
    ):
        self.name = name
        self.failure_threshold = failure_threshold
        self.recovery_timeout = recovery_timeout
        self.half_open_max = half_open_max
        
        self.state = self.CLOSED
        self.failures = 0
        self.successes = 0
        self.last_failure_time: Optional[float] = None
        self.half_open_count = 0
        self._listeners: list = []
    
    def add_listener(self, callback: Callable):
        """Add state change listener for alerting."""
        self._listeners.append(callback)
    
    def _notify_listeners(self, old_state: str, new_state: str):
        for listener in self._listeners:
            try:
                listener(self.name, old_state, new_state)
            except Exception:
                pass
    
    def can_execute(self) -> bool:
        """Check if call should be attempted."""
        if self.state == self.CLOSED:
            return True
        
        if self.state == self.OPEN:
            if self._should_attempt_recovery():
                old_state = self.state
                self.state = self.HALF_OPEN
                self.half_open_count = 0
                self.successes = 0
                self._notify_listeners(old_state, self.state)
                return True
            return False
        
        # HALF_OPEN: allow limited requests
        if self.half_open_count < self.half_open_max:
            self.half_open_count += 1
            return True
        return False
    
    def _should_attempt_recovery(self) -> bool:
        if self.last_failure_time is None:
            return True
        return time.time() - self.last_failure_time >= self.recovery_timeout
    
    def record_success(self):
        """Record successful call."""
        if self.state == self.HALF_OPEN:
            self.successes += 1
            if self.successes >= self.half_open_max:
                old_state = self.state
                self.state = self.CLOSED
                self.failures = 0
                self.successes = 0
                logger.info(f"Circuit '{self.name}' closed - service recovered")
                self._notify_listeners(old_state, self.state)
        elif self.state == self.CLOSED:
            self.failures = 0
    
    def record_failure(self):
        """Record failed call."""
        self.failures += 1
        self.last_failure_time = time.time()
        
        if self.state == self.HALF_OPEN:
            old_state = self.state
            self.state = self.OPEN
            logger.warning(f"Circuit '{self.name}' re-opened - still failing")
            self._notify_listeners(old_state, self.state)
        elif self.failures >= self.failure_threshold:
            old_state = self.state
            self.state = self.OPEN
            logger.error(f"Circuit '{self.name}' opened after {self.failures} failures")
            self._notify_listeners(old_state, self.state)
    
    def get_status(self) -> Dict:
        """Get circuit status for health checks."""
        return {
            "name": self.name,
            "state": self.state,
            "failures": self.failures,
            "last_failure": datetime.fromtimestamp(self.last_failure_time).isoformat() if self.last_failure_time else None
        }

# =============================================================================
# Circuit Breaker Registry
# =============================================================================

class CircuitBreakerRegistry:
    """Global registry of all circuit breakers."""
    
    _instance = None
    
    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
            cls._instance._breakers = {}
            cls._instance._alert_callback = None
        return cls._instance
    
    def get_or_create(
        self,
        name: str,
        failure_threshold: int = 5,
        recovery_timeout: int = 30
    ) -> CircuitBreaker:
        """Get existing or create new circuit breaker."""
        if name not in self._breakers:
            cb = CircuitBreaker(name, failure_threshold, recovery_timeout)
            if self._alert_callback:
                cb.add_listener(self._alert_callback)
            self._breakers[name] = cb
        return self._breakers[name]
    
    def set_alert_callback(self, callback: Callable):
        """Set global alert callback for all breakers."""
        self._alert_callback = callback
        for cb in self._breakers.values():
            cb.add_listener(callback)
    
    def get_all_status(self) -> list:
        """Get status of all breakers."""
        return [cb.get_status() for cb in self._breakers.values()]
    
    def get_degraded(self) -> list:
        """Get list of open/degraded circuits."""
        return [
            cb.get_status() for cb in self._breakers.values()
            if cb.state != CircuitBreaker.CLOSED
        ]

# Global registry singleton
circuits = CircuitBreakerRegistry()

# =============================================================================
# Redis Health Monitor
# =============================================================================

class RedisHealthMonitor:
    """
    Monitors Redis health and triggers alerts on outage.
    """
    
    def __init__(self, redis_client, check_interval: int = 10):
        self.redis = redis_client
        self.check_interval = check_interval
        self.is_healthy = True
        self.last_healthy = time.time()
        self.outage_start: Optional[float] = None
        self._alert_callbacks: list = []
        self._running = False
    
    def add_alert_callback(self, callback: Callable):
        """Add callback for Redis outage alerts."""
        self._alert_callbacks.append(callback)
    
    async def _alert(self, event: str, details: Dict):
        """Trigger all alert callbacks."""
        for callback in self._alert_callbacks:
            try:
                if asyncio.iscoroutinefunction(callback):
                    await callback(event, details)
                else:
                    callback(event, details)
            except Exception as e:
                logger.error(f"Alert callback failed: {e}")
    
    async def check_health(self) -> bool:
        """Check Redis connectivity."""
        if not self.redis:
            return False
        try:
            await self.redis.ping()
            return True
        except Exception:
            return False
    
    async def start_monitoring(self):
        """Start background health monitoring."""
        self._running = True
        logger.info("Redis health monitor started")
        
        while self._running:
            try:
                healthy = await self.check_health()
                
                if healthy and not self.is_healthy:
                    # Recovery
                    outage_duration = time.time() - self.outage_start if self.outage_start else 0
                    await self._alert("redis_recovered", {
                        "timestamp": datetime.utcnow().isoformat(),
                        "outage_duration_seconds": outage_duration
                    })
                    logger.info(f"Redis recovered after {outage_duration:.1f}s outage")
                    self.outage_start = None
                    
                elif not healthy and self.is_healthy:
                    # New outage
                    self.outage_start = time.time()
                    await self._alert("redis_outage", {
                        "timestamp": datetime.utcnow().isoformat(),
                        "message": "Redis connection lost"
                    })
                    logger.error("Redis outage detected!")
                
                self.is_healthy = healthy
                if healthy:
                    self.last_healthy = time.time()
                
                await asyncio.sleep(self.check_interval)
                
            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"Health monitor error: {e}")
                await asyncio.sleep(self.check_interval)
        
        logger.info("Redis health monitor stopped")
    
    def stop(self):
        """Stop monitoring."""
        self._running = False
    
    def get_status(self) -> Dict:
        """Get current Redis health status."""
        return {
            "healthy": self.is_healthy,
            "last_healthy": datetime.fromtimestamp(self.last_healthy).isoformat(),
            "outage_duration": time.time() - self.outage_start if self.outage_start else 0
        }

# =============================================================================
# Resilient HTTP Client Wrapper
# =============================================================================

class ResilientClient:
    """
    HTTP client with circuit breaker and retry logic.
    """
    
    def __init__(
        self,
        http_client,
        service_name: str,
        failure_threshold: int = 3,
        recovery_timeout: int = 20,
        max_retries: int = 2,
        base_delay: float = 0.5
    ):
        self.client = http_client
        self.service_name = service_name
        self.circuit = circuits.get_or_create(
            service_name, failure_threshold, recovery_timeout
        )
        self.max_retries = max_retries
        self.base_delay = base_delay
    
    async def get(self, url: str, **kwargs) -> Any:
        """GET request with resilience."""
        return await self._request("GET", url, **kwargs)
    
    async def post(self, url: str, **kwargs) -> Any:
        """POST request with resilience."""
        return await self._request("POST", url, **kwargs)
    
    async def _request(self, method: str, url: str, **kwargs) -> Any:
        """Execute request with circuit breaker and retry."""
        
        # Check circuit
        if not self.circuit.can_execute():
            raise ServiceUnavailableError(
                f"Service {self.service_name} unavailable (circuit open)"
            )
        
        last_error = None
        
        for attempt in range(self.max_retries + 1):
            try:
                if method == "GET":
                    response = await self.client.get(url, **kwargs)
                else:
                    response = await self.client.post(url, **kwargs)
                
                if response.status_code >= 500:
                    raise ServiceError(f"Server error: {response.status_code}")
                
                self.circuit.record_success()
                return response
                
            except Exception as e:
                last_error = e
                logger.warning(
                    f"{self.service_name} request failed (attempt {attempt + 1}): {e}"
                )
                
                if attempt < self.max_retries:
                    delay = self.base_delay * (2 ** attempt)
                    await asyncio.sleep(delay)
        
        self.circuit.record_failure()
        raise last_error

class ServiceError(Exception):
    """Service returned an error."""
    pass

class ServiceUnavailableError(Exception):
    """Service is unavailable (circuit open)."""
    pass

# =============================================================================
# Decorator for resilient calls
# =============================================================================

def with_circuit_breaker(
    circuit_name: str,
    failure_threshold: int = 3,
    recovery_timeout: int = 20
):
    """Decorator to wrap async function with circuit breaker."""
    
    def decorator(func):
        circuit = circuits.get_or_create(circuit_name, failure_threshold, recovery_timeout)
        
        @wraps(func)
        async def wrapper(*args, **kwargs):
            if not circuit.can_execute():
                raise ServiceUnavailableError(f"Circuit {circuit_name} is open")
            
            try:
                result = await func(*args, **kwargs)
                circuit.record_success()
                return result
            except Exception as e:
                circuit.record_failure()
                raise
        
        return wrapper
    return decorator

# =============================================================================
# Alert System
# =============================================================================

class AlertManager:
    """
    Centralized alert management.
    Stores alerts and can forward to notification service.
    """
    
    def __init__(self, notification_url: Optional[str] = None, redis_client = None):
        self.notification_url = notification_url
        self.redis = redis_client
        self.alerts: list = []
        self._http_client = None
    
    def set_http_client(self, client):
        self._http_client = client
    
    async def send_alert(self, event: str, details: Dict):
        """Send alert to all channels."""
        alert = {
            "event": event,
            "details": details,
            "timestamp": datetime.utcnow().isoformat()
        }
        
        # Store in memory (last 100)
        self.alerts.append(alert)
        if len(self.alerts) > 100:
            self.alerts = self.alerts[-100:]
        
        # Store in Redis
        if self.redis:
            try:
                import json
                await self.redis.lpush("system:alerts", json.dumps(alert))
                await self.redis.ltrim("system:alerts", 0, 99)
            except Exception as e:
                logger.warning(f"Failed to store alert in Redis: {e}")
        
        # Forward to notification service
        if self.notification_url and self._http_client:
            try:
                await self._http_client.post(
                    f"{self.notification_url}/notify",
                    json={
                        "type": "alert",
                        "title": f"System Alert: {event}",
                        "body": str(details),
                        "priority": "high"
                    }
                )
            except Exception as e:
                logger.warning(f"Failed to send notification: {e}")
        
        logger.warning(f"ALERT [{event}]: {details}")
    
    def get_recent_alerts(self, limit: int = 20) -> list:
        """Get recent alerts."""
        return self.alerts[-limit:]
    
    async def get_alerts_from_redis(self, limit: int = 20) -> list:
        """Get alerts from Redis."""
        if not self.redis:
            return self.alerts[-limit:]
        
        try:
            import json
            raw = await self.redis.lrange("system:alerts", 0, limit - 1)
            return [json.loads(a) for a in raw]
        except Exception:
            return self.alerts[-limit:]

# Global alert manager (initialized per service)
alert_manager: Optional[AlertManager] = None

def get_alert_manager() -> Optional[AlertManager]:
    return alert_manager

def init_alert_manager(notification_url: str = None, redis_client = None) -> AlertManager:
    global alert_manager
    alert_manager = AlertManager(notification_url, redis_client)
    
    # Wire up circuit breaker alerts
    async def on_circuit_change(name: str, old_state: str, new_state: str):
        if new_state == CircuitBreaker.OPEN:
            await alert_manager.send_alert("circuit_opened", {
                "circuit": name,
                "message": f"Service {name} is failing"
            })
        elif new_state == CircuitBreaker.CLOSED and old_state == CircuitBreaker.OPEN:
            await alert_manager.send_alert("circuit_closed", {
                "circuit": name,
                "message": f"Service {name} recovered"
            })
    
    circuits.set_alert_callback(lambda n, o, ne: asyncio.create_task(on_circuit_change(n, o, ne)))
    
    return alert_manager
