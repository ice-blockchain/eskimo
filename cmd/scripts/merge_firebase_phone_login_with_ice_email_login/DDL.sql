-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS merge_firebase_phone_login_with_ice_email_login (
                    created_at   timestamp DEFAULT current_timestamp,
                    email        text NOT NULL UNIQUE,
                    phone_number text PRIMARY KEY
                    );
-- Insert here the data that need to be handled.
INSERT INTO merge_firebase_phone_login_with_ice_email_login (phone_number,email)
          VALUES
               ('test', 'test@test.com')
ON CONFLICT DO NOTHING;