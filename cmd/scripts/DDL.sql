-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS update_email_temporary (
                    phone_number text primary key,
                    email text NOT NULL UNIQUE);
-- Insert here the data that need to be handled.
INSERT INTO update_email_temporary (phone_number,email)
          VALUES
               ('test', 'test@test.com')
ON CONFLICT DO NOTHING;