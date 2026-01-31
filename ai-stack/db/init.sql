-- =============================================================================
-- Personal AI Operating System - Database Initialization
-- =============================================================================
-- PostgreSQL 16 + pgvector extension for semantic memory

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- =============================================================================
-- USER PROFILE & IDENTITY
-- =============================================================================

CREATE TABLE IF NOT EXISTS user_profile (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) UNIQUE NOT NULL DEFAULT 'primary',
    name VARCHAR(255),
    preferences JSONB DEFAULT '{}',
    timezone VARCHAR(50) DEFAULT 'UTC',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Insert default user
INSERT INTO user_profile (user_id, name, preferences)
VALUES ('primary', 'User', '{"voice_enabled": true, "notification_level": "normal"}')
ON CONFLICT (user_id) DO NOTHING;

-- =============================================================================
-- CONVERSATION MEMORY
-- =============================================================================

CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    title VARCHAR(500),
    summary TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_message_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    message_count INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID REFERENCES conversations(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content TEXT NOT NULL,
    tokens_used INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id);
CREATE INDEX idx_messages_created ON messages(created_at DESC);

-- =============================================================================
-- SEMANTIC MEMORY (EMBEDDINGS)
-- =============================================================================

CREATE TABLE IF NOT EXISTS memory_embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    source_type VARCHAR(50) NOT NULL, -- 'conversation', 'fact', 'document', 'agent_result'
    source_id UUID,
    content TEXT NOT NULL,
    embedding vector(384), -- all-MiniLM-L6-v2 dimension
    importance_score FLOAT DEFAULT 0.5,
    access_count INTEGER DEFAULT 0,
    last_accessed TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

-- HNSW index for fast similarity search
CREATE INDEX idx_embeddings_vector ON memory_embeddings 
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

CREATE INDEX idx_embeddings_source ON memory_embeddings(source_type, source_id);
CREATE INDEX idx_embeddings_user ON memory_embeddings(user_id);

-- =============================================================================
-- STRUCTURED FACTS (KNOWLEDGE BASE)
-- =============================================================================

CREATE TABLE IF NOT EXISTS facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    category VARCHAR(100) NOT NULL, -- 'personal', 'preference', 'relationship', 'work', 'interest'
    subject VARCHAR(255) NOT NULL,
    predicate VARCHAR(255) NOT NULL,
    object TEXT NOT NULL,
    confidence FLOAT DEFAULT 1.0,
    source VARCHAR(100), -- 'explicit', 'inferred', 'agent'
    valid_from TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    valid_until TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_facts_user_category ON facts(user_id, category);
CREATE INDEX idx_facts_subject ON facts(subject);

-- =============================================================================
-- USER FACTS (for consolidation service archival)
-- =============================================================================

CREATE TABLE IF NOT EXISTS user_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    fact_key VARCHAR(255) NOT NULL,
    fact_value TEXT NOT NULL,
    confidence FLOAT DEFAULT 1.0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_user_facts_active ON user_facts(user_id, is_active);

-- =============================================================================
-- CONVERSATION SUMMARIES (for memory consolidation)
-- =============================================================================

CREATE TABLE IF NOT EXISTS conversation_summaries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID UNIQUE REFERENCES conversations(id) ON DELETE CASCADE,
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    summary TEXT NOT NULL,
    key_topics TEXT[],
    message_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_conv_summaries_user ON conversation_summaries(user_id);
CREATE INDEX idx_conv_summaries_created ON conversation_summaries(created_at DESC);

-- =============================================================================
-- AGENT TASKS & RESULTS
-- =============================================================================

CREATE TABLE IF NOT EXISTS agent_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    agent_type VARCHAR(100) NOT NULL,
    task_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    priority INTEGER DEFAULT 5,
    input_data JSONB DEFAULT '{}',
    output_data JSONB,
    error_message TEXT,
    scheduled_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_agent_tasks_status ON agent_tasks(status, scheduled_at);
CREATE INDEX idx_agent_tasks_user ON agent_tasks(user_id);

-- =============================================================================
-- SCHEDULED JOBS
-- =============================================================================

CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    job_type VARCHAR(100) NOT NULL, -- 'reminder', 'recurring', 'cron', 'one_time'
    name VARCHAR(255) NOT NULL,
    description TEXT,
    cron_expression VARCHAR(100),
    next_run_at TIMESTAMP WITH TIME ZONE,
    last_run_at TIMESTAMP WITH TIME ZONE,
    enabled BOOLEAN DEFAULT true,
    action_type VARCHAR(100) NOT NULL, -- 'notify', 'agent', 'message'
    action_payload JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_scheduled_jobs_next_run ON scheduled_jobs(next_run_at) WHERE enabled = true;

-- =============================================================================
-- INTERESTS & TOPICS (for info-engine)
-- =============================================================================

CREATE TABLE IF NOT EXISTS interests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    topic VARCHAR(255) NOT NULL,
    keywords TEXT[],
    priority INTEGER DEFAULT 5,
    notification_enabled BOOLEAN DEFAULT true,
    last_checked TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_interests_user ON interests(user_id);

-- =============================================================================
-- NOTIFICATIONS
-- =============================================================================

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) REFERENCES user_profile(user_id) DEFAULT 'primary',
    type VARCHAR(50) NOT NULL, -- 'info', 'reminder', 'alert', 'update'
    title VARCHAR(255) NOT NULL,
    body TEXT,
    priority VARCHAR(20) DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    channel VARCHAR(50), -- 'push', 'email', 'bot', 'voice'
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'read', 'dismissed')),
    scheduled_for TIMESTAMP WITH TIME ZONE,
    sent_at TIMESTAMP WITH TIME ZONE,
    read_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_notifications_status ON notifications(status, scheduled_for);
CREATE INDEX idx_notifications_user ON notifications(user_id);

-- =============================================================================
-- AUDIT LOG
-- =============================================================================

CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100),
    entity_id UUID,
    details JSONB DEFAULT '{}',
    ip_address INET,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);
CREATE INDEX idx_audit_log_action ON audit_log(action);

-- =============================================================================
-- HELPER FUNCTIONS
-- =============================================================================

-- Function to update 'updated_at' timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to tables with updated_at
CREATE TRIGGER update_user_profile_updated_at
    BEFORE UPDATE ON user_profile
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_facts_updated_at
    BEFORE UPDATE ON facts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_scheduled_jobs_updated_at
    BEFORE UPDATE ON scheduled_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function for semantic search
CREATE OR REPLACE FUNCTION semantic_search(
    query_embedding vector(384),
    match_threshold FLOAT DEFAULT 0.7,
    match_count INT DEFAULT 10,
    filter_user VARCHAR DEFAULT 'primary'
)
RETURNS TABLE (
    id UUID,
    content TEXT,
    source_type VARCHAR,
    similarity FLOAT,
    metadata JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        me.id,
        me.content,
        me.source_type,
        1 - (me.embedding <=> query_embedding) AS similarity,
        me.metadata
    FROM memory_embeddings me
    WHERE me.user_id = filter_user
      AND 1 - (me.embedding <=> query_embedding) > match_threshold
    ORDER BY me.embedding <=> query_embedding
    LIMIT match_count;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Initial System Records
-- =============================================================================

-- Log system initialization
INSERT INTO audit_log (action, entity_type, details)
VALUES ('system_init', 'database', '{"version": "1.0.0", "initialized_at": "now()"}');

COMMENT ON DATABASE ai_memory IS 'Personal AI Operating System - Memory Database';
