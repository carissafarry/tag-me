-- Minimal seed data for local development (TAG-7)
-- One valid active QR token for testing POST /messages endpoint

-- Test owner (not exposed to scanners)
INSERT INTO qr_codes (owner_id, qr_token, object_type, object_id, is_active)
VALUES (
  '550e8400-e29b-41d4-a716-446655440000'::uuid,
  'demo-token-123',
  'item',
  '660e8400-e29b-41d4-a716-446655440001'::uuid,
  true
);
