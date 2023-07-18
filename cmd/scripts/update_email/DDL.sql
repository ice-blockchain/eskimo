-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS update_email_temporary (
                    handled_at   timestamp,
                    email        text NOT NULL UNIQUE,
                    phone_number text PRIMARY KEY
                    );
-- Insert here the data that need to be handled.
INSERT INTO update_email_temporary (phone_number,email)
          VALUES
               ('test', 'test@gmail.com')
ON CONFLICT DO NOTHING;