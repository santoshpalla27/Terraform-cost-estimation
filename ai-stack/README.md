# Personal AI Operating System

A self-hosted, containerized AI platform with voice interaction, persistent memory, autonomous agents, and multi-client access.

## 🏗 Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Layer 1: Interface Layer                      │
│              (Web UI, Android, Telegram, Voice Devices)          │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                    Layer 2: Gateway Layer                        │
│                   reverse-proxy → api-gateway                    │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                Layer 3: Core Intelligence                        │
│     ai-brain ← memory-service ← ollama-runtime                  │
│     stt-service → tts-service                                   │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│               Layer 4: Autonomous Systems                        │
│  agent-engine → scheduler → info-engine → notifications         │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                Layer 5: Persistence                              │
│            PostgreSQL (pgvector) + Redis                        │
└─────────────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- 8GB+ RAM (16GB recommended)
- 30GB+ disk space

### Deploy

```bash
# Clone and navigate
cd ai-stack

# Configure environment
cp .env.example .env
# Edit .env with your settings

# Start core services (without voice)
docker compose up -d memory-db redis ollama-runtime

# Pull LLM model
docker compose exec ollama-runtime ollama pull qwen2.5:3b

# Start remaining services
docker compose up -d

# With voice services
docker compose --profile voice up -d

# With proactive services
docker compose --profile proactive up -d
```

### Verify

```bash
# Check all services
docker compose ps

# Test API
curl http://localhost/api/

# Test chat
curl -X POST http://localhost/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello!"}'
```

## 📦 Services

| Service | Port | Description |
|---------|------|-------------|
| reverse-proxy | 80/443 | Nginx gateway |
| api-gateway | 8000 | REST/WebSocket API |
| ai-brain | 8001 | LLM orchestration |
| memory-service | 8002 | Semantic memory |
| stt-service | 8003 | Speech-to-text |
| tts-service | 8004 | Text-to-speech |
| agent-engine | 8005 | Task execution |
| scheduler | 8006 | Job scheduling |
| info-engine | 8007 | News/RSS monitoring |
| notifications | 8008 | Alert dispatch |
| ollama-runtime | 11434 | LLM inference |
| memory-db | 5432 | PostgreSQL |
| redis | 6379 | Cache/queue |

## 🧠 AI Configuration

### Model Selection

Edit `.env`:
```
LLM_MODEL=qwen2.5:3b  # Default, fits in 8GB RAM
# LLM_MODEL=qwen2.5:7b  # Better quality, needs 16GB
# LLM_MODEL=llama3.2:3b  # Alternative
```

### Personality

Edit `brain/identity/persona.yaml`:
```yaml
name: Atlas
personality:
  tone: friendly, warm
  traits: [helpful, curious, honest]
```

## 🎤 Voice

Enable voice services:
```bash
docker compose --profile voice up -d
```

STT models (edit `.env`):
- `tiny` - Fastest, ~1GB RAM
- `base` - Better quality, ~1GB RAM  
- `small` - Best local quality, ~2GB RAM

## 🔧 Operations

### Logs
```bash
docker compose logs -f ai-brain
docker compose logs -f --tail 100
```

### Backup
```bash
# Database
docker compose exec memory-db pg_dump -U ai_user ai_memory > backup.sql

# All volumes
docker run --rm -v ai-memory-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/memory-backup.tar.gz /data
```

### Update
```bash
docker compose pull
docker compose up -d --build
```

## 📁 Project Structure

```
ai-stack/
├── docker-compose.yml
├── .env.example
├── nginx/
│   └── nginx.conf
├── api/                  # API Gateway
├── brain/                # AI Brain + Identity
├── memory/               # Memory Service
├── stt/                  # Speech-to-Text
├── tts/                  # Text-to-Speech
├── agents/               # Agent Engine
├── scheduler/            # Job Scheduler
├── info/                 # Info Engine
├── notifications/        # Notification Service
└── db/
    └── init.sql          # Database schema
```

## 📄 License

MIT
