"""
=============================================================================
Personal AI Operating System - Agent Engine
=============================================================================
Layer 4: Autonomous Systems - Task Execution Framework

The Agent Engine executes real-world actions on behalf of the AI.
Agents are plugins that:
- Declare capabilities
- Require permissions
- Log execution
- Run sandboxed

The Brain chooses agents. Agents act.
This separation prevents the Brain from directly touching the outside world.
"""

import os
import logging
from datetime import datetime
from contextlib import asynccontextmanager
from typing import Optional, Dict, Any, Callable
from enum import Enum
import asyncio

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
import httpx
import redis.asyncio as redis

# =============================================================================
# Configuration
# =============================================================================

class Settings:
    REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
    BRAIN_URL = os.getenv("BRAIN_URL", "http://ai-brain:8001")
    MEMORY_URL = os.getenv("MEMORY_URL", "http://memory-service:8002")
    LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")

settings = Settings()

# =============================================================================
# Logging
# =============================================================================

logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("agent-engine")

# =============================================================================
# Models
# =============================================================================

class AgentStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"

class TaskRequest(BaseModel):
    agent_type: str
    task_name: str
    input_data: Dict[str, Any] = {}
    user_id: str = "primary"
    priority: int = 5
    conversation_id: Optional[str] = None

class TaskResponse(BaseModel):
    task_id: str
    status: AgentStatus
    result: Optional[Dict[str, Any]] = None
    error: Optional[str] = None
    started_at: Optional[str] = None
    completed_at: Optional[str] = None

class AgentInfo(BaseModel):
    name: str
    description: str
    capabilities: list
    required_permissions: list

class HealthResponse(BaseModel):
    status: str
    agents_available: int
    queue_size: int
    timestamp: str

# =============================================================================
# Agent Registry
# =============================================================================

class AgentRegistry:
    """Registry of available agents and their handlers."""
    
    def __init__(self):
        self._agents: Dict[str, Dict] = {}
        self._register_builtin_agents()
    
    def _register_builtin_agents(self):
        """Register built-in agents."""
        
        # Reminder Agent
        self.register(
            name="reminder",
            description="Set reminders and notifications",
            capabilities=["set_reminder", "list_reminders", "cancel_reminder"],
            permissions=["notification"],
            handler=self._reminder_handler
        )
        
        # Calculator Agent
        self.register(
            name="calculator",
            description="Perform mathematical calculations",
            capabilities=["calculate", "convert_units"],
            permissions=[],
            handler=self._calculator_handler
        )
        
        # Weather Agent
        self.register(
            name="weather",
            description="Get weather information",
            capabilities=["current_weather", "forecast"],
            permissions=["network"],
            handler=self._weather_handler
        )
        
        # Search Agent
        self.register(
            name="search",
            description="Search the web for information",
            capabilities=["web_search", "summarize"],
            permissions=["network"],
            handler=self._search_handler
        )
        
        # Note Agent
        self.register(
            name="note",
            description="Take and manage notes",
            capabilities=["create_note", "list_notes", "search_notes"],
            permissions=["memory"],
            handler=self._note_handler
        )
    
    def register(
        self,
        name: str,
        description: str,
        capabilities: list,
        permissions: list,
        handler: Callable
    ):
        """Register a new agent."""
        self._agents[name] = {
            "name": name,
            "description": description,
            "capabilities": capabilities,
            "permissions": permissions,
            "handler": handler
        }
        logger.info(f"Registered agent: {name}")
    
    def get(self, name: str) -> Optional[Dict]:
        """Get agent by name."""
        return self._agents.get(name)
    
    def list_all(self) -> list:
        """List all registered agents."""
        return [
            AgentInfo(
                name=a["name"],
                description=a["description"],
                capabilities=a["capabilities"],
                required_permissions=a["permissions"]
            )
            for a in self._agents.values()
        ]
    
    # =========================================================================
    # Built-in Agent Handlers
    # =========================================================================
    
    async def _reminder_handler(self, input_data: Dict, context: Dict) -> Dict:
        """Handle reminder tasks."""
        action = input_data.get("action", "set")
        
        if action == "set":
            message = input_data.get("message", "Reminder")
            time_str = input_data.get("time", "in 1 hour")
            
            # Parse time and schedule via Redis
            redis_client = context.get("redis")
            if redis_client:
                reminder_id = f"reminder:{datetime.utcnow().timestamp()}"
                await redis_client.hset(reminder_id, mapping={
                    "message": message,
                    "time": time_str,
                    "user_id": context.get("user_id", "primary"),
                    "status": "pending"
                })
                await redis_client.expire(reminder_id, 86400 * 7)  # 7 days TTL
                
                return {
                    "success": True,
                    "reminder_id": reminder_id,
                    "message": f"Reminder set: '{message}' for {time_str}"
                }
        
        elif action == "list":
            return {"success": True, "reminders": []}
        
        return {"success": False, "error": "Unknown action"}
    
    async def _calculator_handler(self, input_data: Dict, context: Dict) -> Dict:
        """Handle calculation tasks."""
        expression = input_data.get("expression", "")
        
        try:
            # Safe evaluation (limited operations)
            allowed_chars = set("0123456789+-*/.() ")
            if not all(c in allowed_chars for c in expression):
                return {"success": False, "error": "Invalid characters in expression"}
            
            result = eval(expression)  # Only for simple math
            return {"success": True, "result": result, "expression": expression}
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    async def _weather_handler(self, input_data: Dict, context: Dict) -> Dict:
        """Handle weather tasks (stub - needs API key)."""
        location = input_data.get("location", "")
        
        # This would call a weather API
        return {
            "success": True,
            "location": location,
            "temperature": "Not configured",
            "note": "Weather API integration required"
        }
    
    async def _search_handler(self, input_data: Dict, context: Dict) -> Dict:
        """Handle search tasks (stub - needs API key)."""
        query = input_data.get("query", "")
        
        return {
            "success": True,
            "query": query,
            "results": [],
            "note": "Search API integration required"
        }
    
    async def _note_handler(self, input_data: Dict, context: Dict) -> Dict:
        """Handle note tasks."""
        action = input_data.get("action", "create")
        
        http_client = context.get("http_client")
        user_id = context.get("user_id", "primary")
        
        if action == "create" and http_client:
            content = input_data.get("content", "")
            title = input_data.get("title", content[:30])
            
            try:
                resp = await http_client.post(
                    f"{settings.MEMORY_URL}/facts",
                    json={
                        "user_id": user_id,
                        "category": "note",
                        "subject": title,
                        "predicate": "contains",
                        "object": content,
                        "source": "agent"
                    }
                )
                if resp.status_code == 200:
                    return {"success": True, "message": f"Note saved: {title}"}
            except Exception as e:
                logger.error(f"Note save failed: {e}")
        
        return {"success": False, "error": "Operation failed"}

agent_registry = AgentRegistry()

# =============================================================================
# Task Queue
# =============================================================================

class TaskQueue:
    """Task queue using Redis."""
    
    def __init__(self, redis_client):
        self.redis = redis_client
        self.queue_key = "agent:task_queue"
        self.results_prefix = "agent:result:"
    
    async def enqueue(self, task_id: str, task_data: Dict):
        """Add task to queue."""
        import json
        await self.redis.lpush(self.queue_key, json.dumps({
            "task_id": task_id,
            **task_data
        }))
    
    async def dequeue(self, timeout: int = 1) -> Optional[Dict]:
        """Get next task from queue."""
        import json
        result = await self.redis.brpop(self.queue_key, timeout=timeout)
        if result:
            return json.loads(result[1])
        return None
    
    async def store_result(self, task_id: str, result: Dict, ttl: int = 3600):
        """Store task result."""
        import json
        await self.redis.setex(
            f"{self.results_prefix}{task_id}",
            ttl,
            json.dumps(result)
        )
    
    async def get_result(self, task_id: str) -> Optional[Dict]:
        """Get task result."""
        import json
        result = await self.redis.get(f"{self.results_prefix}{task_id}")
        if result:
            return json.loads(result)
        return None
    
    async def queue_size(self) -> int:
        """Get current queue size."""
        return await self.redis.llen(self.queue_key)

# =============================================================================
# Application Lifecycle
# =============================================================================

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifecycle."""
    logger.info("🤖 Agent Engine starting up...")
    
    # Initialize Redis
    try:
        app.state.redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        await app.state.redis.ping()
        app.state.task_queue = TaskQueue(app.state.redis)
        logger.info("✅ Redis connected")
    except Exception as e:
        logger.warning(f"⚠️ Redis unavailable: {e}")
        app.state.redis = None
        app.state.task_queue = None
    
    # Initialize HTTP client
    app.state.http_client = httpx.AsyncClient(timeout=60.0)
    
    # Start background worker
    app.state.worker_task = asyncio.create_task(task_worker(app))
    
    yield
    
    logger.info("🛑 Agent Engine shutting down...")
    app.state.worker_task.cancel()
    await app.state.http_client.aclose()
    if app.state.redis:
        await app.state.redis.close()

async def task_worker(app: FastAPI):
    """Background worker for processing tasks."""
    logger.info("Worker started")
    
    while True:
        try:
            if not app.state.task_queue:
                await asyncio.sleep(5)
                continue
            
            task = await app.state.task_queue.dequeue(timeout=1)
            
            if task:
                await process_task(app, task)
            
        except asyncio.CancelledError:
            break
        except Exception as e:
            logger.error(f"Worker error: {e}")
            await asyncio.sleep(1)

async def process_task(app: FastAPI, task: Dict):
    """Process a single task."""
    task_id = task.get("task_id")
    agent_type = task.get("agent_type")
    input_data = task.get("input_data", {})
    user_id = task.get("user_id", "primary")
    
    logger.info(f"Processing task {task_id}: {agent_type}")
    
    agent = agent_registry.get(agent_type)
    if not agent:
        await app.state.task_queue.store_result(task_id, {
            "status": "failed",
            "error": f"Unknown agent: {agent_type}"
        })
        return
    
    try:
        # Execute agent handler
        context = {
            "redis": app.state.redis,
            "http_client": app.state.http_client,
            "user_id": user_id
        }
        
        result = await agent["handler"](input_data, context)
        
        await app.state.task_queue.store_result(task_id, {
            "status": "completed",
            "result": result,
            "completed_at": datetime.utcnow().isoformat()
        })
        
    except Exception as e:
        logger.error(f"Task {task_id} failed: {e}")
        await app.state.task_queue.store_result(task_id, {
            "status": "failed",
            "error": str(e)
        })

# =============================================================================
# FastAPI Application
# =============================================================================

app = FastAPI(
    title="Personal AI OS - Agent Engine",
    description="Task Execution Framework",
    version="1.0.0",
    lifespan=lifespan
)

# =============================================================================
# Endpoints
# =============================================================================

@app.get("/health", response_model=HealthResponse)
async def health_check():
    """Health check endpoint."""
    queue_size = 0
    if app.state.task_queue:
        queue_size = await app.state.task_queue.queue_size()
    
    return HealthResponse(
        status="healthy",
        agents_available=len(agent_registry.list_all()),
        queue_size=queue_size,
        timestamp=datetime.utcnow().isoformat()
    )

@app.get("/agents")
async def list_agents():
    """List available agents."""
    return {"agents": [a.model_dump() for a in agent_registry.list_all()]}

@app.get("/agents/{agent_name}")
async def get_agent(agent_name: str):
    """Get agent details."""
    agent = agent_registry.get(agent_name)
    if not agent:
        raise HTTPException(status_code=404, detail="Agent not found")
    
    return AgentInfo(
        name=agent["name"],
        description=agent["description"],
        capabilities=agent["capabilities"],
        required_permissions=agent["permissions"]
    )

@app.post("/execute", response_model=TaskResponse)
async def execute_task(request: TaskRequest):
    """Execute an agent task."""
    import uuid
    
    agent = agent_registry.get(request.agent_type)
    if not agent:
        raise HTTPException(status_code=404, detail=f"Agent not found: {request.agent_type}")
    
    task_id = str(uuid.uuid4())
    
    if app.state.task_queue:
        # Async execution via queue
        await app.state.task_queue.enqueue(task_id, {
            "agent_type": request.agent_type,
            "task_name": request.task_name,
            "input_data": request.input_data,
            "user_id": request.user_id,
            "priority": request.priority
        })
        
        return TaskResponse(
            task_id=task_id,
            status=AgentStatus.PENDING,
            started_at=datetime.utcnow().isoformat()
        )
    else:
        # Sync execution (no Redis)
        context = {
            "redis": None,
            "http_client": app.state.http_client,
            "user_id": request.user_id
        }
        
        try:
            result = await agent["handler"](request.input_data, context)
            return TaskResponse(
                task_id=task_id,
                status=AgentStatus.COMPLETED,
                result=result,
                completed_at=datetime.utcnow().isoformat()
            )
        except Exception as e:
            return TaskResponse(
                task_id=task_id,
                status=AgentStatus.FAILED,
                error=str(e)
            )

@app.get("/tasks/{task_id}", response_model=TaskResponse)
async def get_task_status(task_id: str):
    """Get task status and result."""
    if not app.state.task_queue:
        raise HTTPException(status_code=503, detail="Task queue unavailable")
    
    result = await app.state.task_queue.get_result(task_id)
    
    if not result:
        return TaskResponse(
            task_id=task_id,
            status=AgentStatus.PENDING
        )
    
    return TaskResponse(
        task_id=task_id,
        status=AgentStatus(result.get("status", "pending")),
        result=result.get("result"),
        error=result.get("error"),
        completed_at=result.get("completed_at")
    )

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8005)
