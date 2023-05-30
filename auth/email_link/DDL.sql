-- SPDX-License-Identifier: ice License 1.0

CREATE TABLE IF NOT EXISTS email_confirmations  (
           created_at timestamp NOT NULL,
           token_issued_at timestamp,
           issued_token_seq BIGINT DEFAULT 0,
           email TEXT NOT NULL primary key,
           otp   TEXT NOT NULL,
           user_id TEXT,
           custom_claims    JSONB
);