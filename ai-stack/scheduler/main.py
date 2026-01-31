"""
=============================================================================
Personal AI Operating System - Scheduler Service
=============================================================================
Layer 4: Autonomous Systems - Time-Based Job Execution

The Scheduler is the heartbeat of the system.
Handles:
- Reminders
- Cron jobs
- Recurring tasks
- Polling cycles
- Agent triggers

Without scheduler: AI is reactive only.
With scheduler: AI becomes autonomous.
"""

import os
import logging
from datetime import datetime, timedelta
from contextlib import asynccontextmanager
from typing import Optional, Dict, Any, List
import asyncio
import json

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from croniter import croniter
import httpx
import redis.asyncio as redis

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    AGENT_ENGINE_URL = os.getenv("AGENT_ENGINE_URL", "http://agent-engine:8005")
    MEMORY_URL = os.getenv("MEMORY_URL", "http://memory-service:8002")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
    CHECK_INTERVAL = int(os.getenv("CHECK_INTERVAL", "10"))  # seconds

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("scheduler")

# =============================================================================
# Models
# =============================================================================

class JobType(str):
    REMINDER = "reminder"
    RECURRING = "recurring"
    CRON = "cron"
    ONE_TIME = "one_time"

class ActionType(str):
    NOTIFY = "notify"
    AGENT = "agent"
    MESSAGE = "message"

class JobRequest(BaseModel):
    job_type: str = Field(..., pattern="^(reminder|recurring|cron|one_time)$")
    name: str
    description: Optional[str] = None
    cron_expression: Optional[str] = None  # For cron jobs
    run_at: Optional[str] = None  # ISO datetime for one-time
    interval_minutes: Optional[int] = None  # For recurring
    action_type: str = Field(..., pattern="^(notify|agent|message)$")
    action_payload: Dict[str, Any] = {}
    user_id: str = "primary"

class JobResponse(BaseModel):
    job_id: str
    name: str
    job_type: str
    next_run_at: Optional[str]
    enabled: bool
    created_at: str

class HealthResponse(BaseModel):
    status: str
    active_jobs: int
    next_execution: Optional[str]
    timestamp: str

# =============================================================================
# Job Store (Redis-backed)
# =============================================================================

class JobStore:
    """Redis-based job storage."""
    
    JOBS_KEY = "scheduler:jobs"
    DUE_JOBS_KEY = "scheduler:due"
    
    def __init__(self, redis_client):
        self.redis = redis_client
    
    async def add_job(self, job_id: str, job_data: Dict) -> None:
        """Add a job to the store."""
        await self.redis.hset(self.JOBS_KEY, job_id, json.dumps(job_data))
        
        # Add to due queue if has next_run_at
        if job_data.get("next_run_at"):
            score = datetime.fromisoformat(job_data["next_run_at"]).timestamp()
            await self.redis.zadd(self.DUE_JOBS_KEY, {job_id: score})
    
    async def get_job(self, job_id: str) -> Optional[Dict]:
        """Get a job by ID."""
        data = await self.redis.hget(self.JOBS_KEY, job_id)
        return json.loads(data) if data else None
    
    async def update_job(self, job_id: str, updates: Dict) -> None:
        """Update a job."""
        job = await self.get_job(job_id)
        if job:
            job.update(updates)
            await self.redis.hset(self.JOBS_KEY, job_id, json.dumps(job))
            
            if updates.get("next_run_at"):
                score = datetime.fromisoformat(updates["next_run_at"]).timestamp()
                await self.redis.zadd(self.DUE_JOBS_KEY, {job_id: score})
    
    async def delete_job(self, job_id: str) -> None:
        """Delete a job."""
        await self.redis.hdel(self.JOBS_KEY, job_id)
        await self.redis.zrem(self.DUE_JOBS_KEY, job_id)
    
    async def get_due_jobs(self, now: datetime) -> List[str]:
        """Get jobs due for execution."""
        score = now.timestamp()
        job_ids = await self.redis.zrangebyscore(self.DUE_JOBS_KEY, 0, score)
        return job_ids
    
    async def get_all_jobs(self, user_id: Optional[str] = None) -> List[Dict]:
        """Get all jobs, optionally filtered by user."""
        all_jobs = await self.redis.hgetall(self.JOBS_KEY)
        jobs = [json.loads(v) for v in all_jobs.values()]
        
        if user_id:
            jobs = [j for j in jobs if j.get("user_id") == user_id]
        
        return jobs
    
    async def get_next_execution(self) -> Optional[datetime]:
        """Get timestamp of next scheduled execution."""
        result = await self.redis.zrange(self.DUE_JOBS_KEY, 0, 0, withscores=True)
        if result:
            return datetime.fromtimestamp(result[0][1])
        return None
    
    async def count_active(self) -> int:
        """Count active jobs."""
        return await self.redis.hlen(self.JOBS_KEY)

# =============================================================================
# Scheduler Logic
# =============================================================================

def calculate_next_run(job: Dict) -> Optional[datetime]:
    """Calculate next run time for a job."""
    now = datetime.utcnow()
    job_type = job.get("job_type")
    
    if job_type == "cron" and job.get("cron_expression"):
        cron = croniter(job["cron_expression"], now)
        return cron.get_next(datetime)
    
    elif job_type == "recurring" and job.get("interval_minutes"):
        interval = timedelta(minutes=job["interval_minutes"])
        last_run = job.get("last_run_at")
        if last_run:
            last = datetime.fromisoformat(last_run)
            return last + interval
        return now + interval
    
    elif job_type == "one_time" and job.get("run_at"):
        run_at = datetime.fromisoformat(job["run_at"])
        if run_at > now:
            return run_at
        return None  # Already passed
    
    elif job_type == "reminder":
        # Simple implementation - should parse natural language
        return now + timedelta(hours=1)
    
    return None

async def execute_job(job: Dict, http_client: httpx.AsyncClient) -> bool:
    """Execute a scheduled job."""
    action_type = job.get("action_type")
    payload = job.get("action_payload", {})
    
    logger.info(f"Executing job: {job.get('name')} ({action_type})")
    
    try:
        if action_type == "agent":
            # Trigger agent
            response = await http_client.post(
                f"{settings.AGENT_ENGINE_URL}/execute",
                json={
                    "agent_type": payload.get("agent_type", "reminder"),
                    "task_name": job.get("name"),
                    "input_data": payload.get("input_data", {}),
                    "user_id": job.get("user_id", "primary")
                }
            )
            return response.status_code == 200
        
        elif action_type == "notify":
            # Store notification in memory/cache
            # In a full implementation, this would go to notification service
            logger.info(f"Notification: {payload.get('message', job.get('name'))}")
            return True
        
        elif action_type == "message":
            # Could trigger a proactive message to user
            logger.info(f"Message: {payload.get('content', '')}")
            return True
        
        return False
        
    except Exception as e:
        logger.error(f"Job execution failed: {e}")
        return False

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("⏰ Scheduler starting up...")
    
    # Initialize Redis
    try:
        app.state.redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        await app.state.redis.ping()
        app.state.job_store = JobStore(app.state.redis)
        logger.info("✅ Redis connected")
    except Exception as e:
        logger.warning(f"⚠️ Redis unavailable: {e}")
        app.state.redis = None
        app.state.job_store = None
    
    # Initialize HTTP client
    app.state.http_client = httpx.AsyncClient(timeout=60.0)
    
    # Start scheduler loop
    app.state.scheduler_task = asyncio.create_task(scheduler_loop(app))
    
    yield
    
    logger.info("🛑 Scheduler shutting down...")
    app.state.scheduler_task.cancel()
    await app.state.http_client.aclose()
    if app.state.redis:
        await app.state.redis.close()

async def scheduler_loop(app: FastAPI):
    """Main scheduler loop - checks for due jobs with adaptive backoff."""
    logger.info("Scheduler loop started")
    
    # Adaptive backoff state
    consecutive_failures = 0
    max_backoff = 300  # 5 minutes max
    base_interval = settings.CHECK_INTERVAL
    
    while True:
        try:
            if not app.state.job_store:
                # Redis unavailable - exponential backoff
                consecutive_failures += 1
                backoff = min(base_interval * (2 ** consecutive_failures), max_backoff)
                logger.warning(f"Redis unavailable, backing off for {backoff}s (failures: {consecutive_failures})")
                await asyncio.sleep(backoff)
                continue
            
            now = datetime.utcnow()
            due_job_ids = await app.state.job_store.get_due_jobs(now)
            
            # Track job execution failures separately
            job_failures = 0
            
            for job_id in due_job_ids:
                job = await app.state.job_store.get_job(job_id)
                if job and job.get("enabled", True):
                    # Execute job
                    success = await execute_job(job, app.state.http_client)
                    
                    if not success:
                        job_failures += 1
                    
                    # Update job
                    job["last_run_at"] = now.isoformat()
                    job["last_run_success"] = success
                    job["failure_count"] = job.get("failure_count", 0) + (0 if success else 1)
                    
                    # Calculate next run
                    next_run = calculate_next_run(job)
                    if next_run:
                        job["next_run_at"] = next_run.isoformat()
                        await app.state.job_store.update_job(job_id, job)
                    else:
                        # One-time job completed
                        if job.get("job_type") == "one_time":
                            await app.state.job_store.delete_job(job_id)
                        else:
                            job["enabled"] = False
                            await app.state.job_store.update_job(job_id, job)
            
            # Reset backoff on successful cycle
            if consecutive_failures > 0:
                logger.info(f"Scheduler recovered after {consecutive_failures} failures")
            consecutive_failures = 0
            
            # Adjust interval based on job execution health
            if job_failures > len(due_job_ids) / 2 and due_job_ids:
                # More than half failed - brief backoff
                await asyncio.sleep(base_interval * 2)
            else:
                await asyncio.sleep(base_interval)
            
        except asyncio.CancelledError:
            break
        except Exception as e:
            consecutive_failures += 1
            backoff = min(base_interval * (2 ** min(consecutive_failures, 6)), max_backoff)
            logger.error(f"Scheduler error (attempt {consecutive_failures}): {e}")
            await asyncio.sleep(backoff)

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - Scheduler",
    description="Time-Based Job Execution",
    version="1.0.0",
    lifespan=lifespan
)

# =============================================================================
# Endpoints
# =============================================================================

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    active_jobs = 0
    next_exec = None
    
    if app.state.job_store:
        active_jobs = await app.state.job_store.count_active()
        next_dt = await app.state.job_store.get_next_execution()
        next_exec = next_dt.isoformat() if next_dt else None
    
    return HealthResponse(
        status="healthy",
        active_jobs=active_jobs,
        next_execution=next_exec,
        timestamp=datetime.utcnow().isoformat()
    )

@app.post("/jobs", response_model=JobResponse)
async def create_job(request: JobRequest):
    """Create a new scheduled job."""
    if not app.state.job_store:
        raise HTTPException(status_code=503, detail="Scheduler unavailable")
    
    import uuid
    job_id = str(uuid.uuid4())
    now = datetime.utcnow()
    
    job_data = {
        "job_id": job_id,
        "name": request.name,
        "description": request.description,
        "job_type": request.job_type,
        "cron_expression": request.cron_expression,
        "run_at": request.run_at,
        "interval_minutes": request.interval_minutes,
        "action_type": request.action_type,
        "action_payload": request.action_payload,
        "user_id": request.user_id,
        "enabled": True,
        "created_at": now.isoformat()
    }
    
    # Calculate next run
    next_run = calculate_next_run(job_data)
    job_data["next_run_at"] = next_run.isoformat() if next_run else None
    
    await app.state.job_store.add_job(job_id, job_data)
    
    return JobResponse(
        job_id=job_id,
        name=request.name,
        job_type=request.job_type,
        next_run_at=job_data["next_run_at"],
        enabled=True,
        created_at=job_data["created_at"]
    )

@app.get("/jobs")
async def list_jobs(user_id: Optional[str] = None):
    """List all scheduled jobs."""
    if not app.state.job_store:
        raise HTTPException(status_code=503, detail="Scheduler unavailable")
    
    jobs = await app.state.job_store.get_all_jobs(user_id)
    return {"jobs": jobs}

@app.get("/jobs/{job_id}")
async def get_job(job_id: str):
    """Get job details."""
    if not app.state.job_store:
        raise HTTPException(status_code=503, detail="Scheduler unavailable")
    
    job = await app.state.job_store.get_job(job_id)
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")
    
    return job

@app.delete("/jobs/{job_id}")
async def delete_job(job_id: str):
    """Delete a job."""
    if not app.state.job_store:
        raise HTTPException(status_code=503, detail="Scheduler unavailable")
    
    await app.state.job_store.delete_job(job_id)
    return {"status": "deleted", "job_id": job_id}

@app.put("/jobs/{job_id}/enable")
async def enable_job(job_id: str):
    """Enable a job."""
    if not app.state.job_store:
        raise HTTPException(status_code=503, detail="Scheduler unavailable")
    
    await app.state.job_store.update_job(job_id, {"enabled": True})
    return {"status": "enabled", "job_id": job_id}

@app.put("/jobs/{job_id}/disable")
async def disable_job(job_id: str):
    """Disable a job."""
    if not app.state.job_store:
        raise HTTPException(status_code=503, detail="Scheduler unavailable")
    
    await app.state.job_store.update_job(job_id, {"enabled": False})
    return {"status": "disabled", "job_id": job_id}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8006)
