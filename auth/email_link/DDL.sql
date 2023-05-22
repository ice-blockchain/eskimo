-- SPDX-License-Identifier: ice License 1.0

CREATE TABLE IF NOT EXISTS pending_email_confirmations  (
           created_at timestamp NOT NULL,
           email TEXT NOT NULL primary key,
           otp   TEXT NOT NULL
        );
CREATE INDEX IF NOT EXISTS users_email_otp_ix ON pending_email_confirmations (email, otp);