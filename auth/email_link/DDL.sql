-- SPDX-License-Identifier: ice License 1.0

CREATE TABLE IF NOT EXISTS pending_email_confirmations  (
           created_at timestamp NOT NULL,
           email TEXT NOT NULL,
           otp   TEXT NOT NULL,
primary key(email, otp));
