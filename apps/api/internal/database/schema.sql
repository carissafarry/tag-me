-- Drop tables (order matters foreign key)
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS qr_codes;

-- Conversations: group of messages around a QR code scan
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    qr_code_id UUID NOT NULL,
    owner_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
);

CREATE INDEX idx_conversations_qr_code_id ON conversations(qr_code_id);
CREATE INDEX idx_conversations_owner_id ON conversations(owner_id);

-- Messages: individual scanner messages within a conversation
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_type VARCHAR(50) NOT NULL DEFAULT 'SCANNER',
    message_type VARCHAR(100) NOT NULL,
    content TEXT,
    location_latitude DECIMAL(10, 8),
    location_longitude DECIMAL(11, 8),
    location_text TEXT,
    session_id VARCHAR(255),
    ip_address INET,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_session_id ON messages(session_id);

-- QR Codes mapping: maps qr_token to owner and object context
CREATE TABLE qr_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    qr_token VARCHAR(255) NOT NULL UNIQUE,
    object_type VARCHAR(100) NOT NULL,
    object_id UUID NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    plate VARCHAR(10) UNIQUE,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
);

CREATE INDEX idx_qr_codes_qr_token ON qr_codes(qr_token);
CREATE INDEX idx_qr_codes_owner_id ON qr_codes(owner_id);
