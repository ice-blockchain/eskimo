-- SPDX-License-Identifier: ice License 1.0

CREATE TABLE IF NOT EXISTS email_link_sign_ins (
           created_at                             timestamp NOT NULL,
           token_issued_at                        timestamp,
           blocked_until                          timestamp,
           email_confirmed_at                     timestamp,
           issued_token_seq                       BIGINT DEFAULT 0 NOT NULL,
           confirmation_code_wrong_attempts_count BIGINT DEFAULT 0 NOT NULL,
           email                                  TEXT NOT NULL,
           otp                                    TEXT NOT NULL,
           confirmation_code                      TEXT,
           user_id                                TEXT,
           device_unique_id                       TEXT,
           primary key(email, device_unique_id))
           WITH (FILLFACTOR = 70);
CREATE INDEX IF NOT EXISTS email_link_sign_ins_user_id_ix ON email_link_sign_ins (user_id);

CREATE TABLE IF NOT EXISTS user_metadata (
    user_id                                TEXT NOT NULL PRIMARY KEY,
    metadata                               JSONB NOT NULL
) WITH (FILLFACTOR = 70);
