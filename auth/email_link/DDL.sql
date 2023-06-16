-- SPDX-License-Identifier: ice License 1.0

CREATE TABLE IF NOT EXISTS email_link_sign_ins (
           created_at                             timestamp NOT NULL,
           confirmation_code_created_at           timestamp,
           token_issued_at                        timestamp,
           issued_token_seq                       BIGINT DEFAULT 0,
           confirmation_code_wrong_attempts_count BIGINT DEFAULT 0,
           email                                  TEXT NOT NULL,
           otp                                    TEXT NOT NULL,
           language                               TEXT NOT NULL DEFAULT 'en',
           login_session                          TEXT,
           confirmation_code                      TEXT,
           user_id                                TEXT,
           device_unique_id                       TEXT,
           custom_claims                          JSONB,
           primary key(email, device_unique_id))
           WITH (FILLFACTOR = 70);
