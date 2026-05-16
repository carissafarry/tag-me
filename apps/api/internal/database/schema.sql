-- Drop tables (order matters for foreign keys)
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS qr_codes;
DROP TABLE IF EXISTS objects;
DROP TABLE IF EXISTS owners;

-- Owners: authenticated users who register objects and receive alerts
CREATE TABLE owners (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact      VARCHAR(255) NOT NULL UNIQUE,
    contact_type VARCHAR(20) NOT NULL DEFAULT 'phone'
        CHECK (contact_type IN ('phone', 'email')),
    is_active  BOOLEAN NOT NULL DEFAULT true,
    dnd_enabled  BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
    updated_at   TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
);

CREATE INDEX idx_owners_contact ON owners(contact);
CREATE INDEX idx_owners_active ON owners(is_active);

-- Objects: physical items owned and tagged with QR codes (vehicles, bags, etc.)
-- ON DELETE CASCADE: deleting an owner deletes their objects; deleting an object deletes its QR code and conversations
CREATE TABLE objects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    object_type VARCHAR(100) NOT NULL
        CHECK (object_type IN ('car', 'motorcycle', 'bag', 'other')),
    plate       VARCHAR(10) UNIQUE,
    created_at  TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
    updated_at  TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
);

CREATE INDEX idx_objects_owner_id ON objects(owner_id);

-- QR Codes: maps qr_token to owner and object (1-to-1 with object for MVP)
-- ON DELETE CASCADE: deleting an object deletes its QR code; deleting an owner deletes their objects and QR codes
-- Post-MVP: drop CONSTRAINT uq_qr_object to allow replacement QR stickers
CREATE TABLE qr_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    object_id  UUID NOT NULL REFERENCES objects(id) ON DELETE CASCADE,
    qr_token   VARCHAR(255) NOT NULL UNIQUE,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
    CONSTRAINT uq_qr_object UNIQUE (object_id)
);

CREATE INDEX idx_qr_codes_qr_token  ON qr_codes(qr_token);
CREATE INDEX idx_qr_codes_owner_id  ON qr_codes(owner_id);
CREATE INDEX idx_qr_codes_object_id ON qr_codes(object_id);

-- Conversations: group of messages around a QR code scan
-- ON DELETE RESTRICT on qr_code_id: prevents deleting a qr_code if conversations exist
-- App enforces: block DELETE /objects/:id if any conversation is active (not RESOLVED/EXPIRED)
-- App enforces order for allowed deletes: conversations → qr_codes → object
CREATE TABLE conversations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    qr_code_id    UUID NOT NULL REFERENCES qr_codes(id) ON DELETE RESTRICT,
    owner_id      UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    status        VARCHAR(50) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'DELIVERED', 'OPENED', 'ON_THE_WAY', 'RESOLVED', 'EXPIRED')),
    expires_at    TIMESTAMP WITHOUT TIME ZONE NOT NULL
        DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta') + interval '24 hours',
    opened_at     TIMESTAMP WITHOUT TIME ZONE,
    on_the_way_at TIMESTAMP WITHOUT TIME ZONE,
    resolved_at   TIMESTAMP WITHOUT TIME ZONE,
    created_at    TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
    updated_at    TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
);

CREATE INDEX idx_conversations_qr_code_id    ON conversations(qr_code_id);
CREATE INDEX idx_conversations_owner_id      ON conversations(owner_id);
CREATE INDEX idx_conversations_owner_status  ON conversations(owner_id, status);
CREATE INDEX idx_conversations_expiry_check  ON conversations(owner_id, expires_at)
    WHERE status NOT IN ('RESOLVED', 'EXPIRED');

-- Messages: individual scanner messages within a conversation
CREATE TABLE messages (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id    UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_type        VARCHAR(50)  NOT NULL DEFAULT 'SCANNER',
    message_type       VARCHAR(100) NOT NULL,
    content            TEXT,
    location_latitude  DECIMAL(10, 8),
    location_longitude DECIMAL(11, 8),
    location_text      TEXT,
    session_id         VARCHAR(255),
    ip_address         INET,
    created_at         TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_session_id      ON messages(session_id);
