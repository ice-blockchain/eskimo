-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS social_kyc_unsuccessful_attempts  (
                    created_at                timestamp NOT NULL,
                    kyc_step                  smallint  NOT NULL CHECK (kyc_step = 3 OR kyc_step = 5 OR kyc_step = 6 OR kyc_step = 7 OR kyc_step = 8 OR kyc_step = 9 OR kyc_step = 10),
                    reason                    text      NOT NULL,
                    user_id                   text      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    PRIMARY KEY (user_id, kyc_step, created_at));

ALTER TABLE social_kyc_unsuccessful_attempts
DROP CONSTRAINT social_kyc_unsuccessful_attempts_kyc_step_check;
ALTER TABLE social_kyc_unsuccessful_attempts
ADD CONSTRAINT social_kyc_unsuccessful_attempts_kyc_step_check
CHECK (kyc_step = 3 OR kyc_step = 5 OR kyc_step = 6 OR kyc_step = 7 OR kyc_step = 8 OR kyc_step = 9 OR kyc_step = 10);

CREATE INDEX IF NOT EXISTS social_kyc_unsuccessful_attempts_lookup1_ix ON social_kyc_unsuccessful_attempts (kyc_step,social,created_at DESC);
CREATE INDEX IF NOT EXISTS social_kyc_unsuccessful_attempts_lookup2_ix ON social_kyc_unsuccessful_attempts (kyc_step,social,created_at DESC,(CASE
                                                                                                                                                WHEN reason like 'duplicate userhandle %' THEN 'duplicate userhandle'
                                                                                                                                                WHEN reason like 'duplicate socials %' THEN 'duplicate socials'
                                                                                                                                                WHEN reason like '%: %' THEN substring(reason from position(': ' in reason) + 2)
                                                                                                                                                ELSE reason
                                                                                                                                            END));

CREATE TABLE IF NOT EXISTS social_kyc_steps (
                    created_at                timestamp NOT NULL,
                    kyc_step                  smallint  NOT NULL CHECK (kyc_step = 3 OR kyc_step = 5 OR kyc_step = 6 OR kyc_step = 7 OR kyc_step = 8 OR kyc_step = 9 OR kyc_step = 10),
                    user_id                   text      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    user_handle               text      NOT NULL,
                    PRIMARY KEY (user_id, kyc_step));

ALTER TABLE social_kyc_steps
DROP CONSTRAINT social_kyc_steps_kyc_step_check;
ALTER TABLE social_kyc_steps
ADD CONSTRAINT social_kyc_steps_kyc_step_check
CHECK (kyc_step = 3 OR kyc_step = 5 OR kyc_step = 6 OR kyc_step = 7 OR kyc_step = 8 OR kyc_step = 9 OR kyc_step = 10);

CREATE INDEX IF NOT EXISTS social_kyc_steps_lookup1_ix ON social_kyc_steps (kyc_step,social,created_at DESC);

CREATE TABLE IF NOT EXISTS socials (
                    user_id                   text      NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    user_handle               text      NOT NULL,
                    UNIQUE (social, user_handle));

CREATE TABLE IF NOT EXISTS unsuccessful_social_kyc_alerts (
                    last_alert_at             timestamp NOT NULL,
                    frequency_in_seconds      bigint    NOT NULL DEFAULT 300,
                    kyc_step                  smallint  NOT NULL CHECK (kyc_step = 3 OR kyc_step = 5 OR kyc_step = 6 OR kyc_step = 7 OR kyc_step = 8 OR kyc_step = 9 OR kyc_step = 10),
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    PRIMARY KEY (kyc_step, social))
                    WITH (FILLFACTOR = 70);

ALTER TABLE unsuccessful_social_kyc_alerts
DROP CONSTRAINT unsuccessful_social_kyc_alerts_kyc_step_check;
ALTER TABLE unsuccessful_social_kyc_alerts
ADD CONSTRAINT unsuccessful_social_kyc_alerts_kyc_step_check
CHECK (kyc_step = 3 OR kyc_step = 5 OR kyc_step = 6 OR kyc_step = 7 OR kyc_step = 8 OR kyc_step = 9 OR kyc_step = 10);

insert into unsuccessful_social_kyc_alerts (last_alert_at,    kyc_step,social)
                                    VALUES (current_timestamp,3,      'twitter'),
                                           (current_timestamp,5,      'twitter'),
                                           (current_timestamp,6,      'twitter'),
                                           (current_timestamp,7,      'twitter'),
                                           (current_timestamp,8,      'twitter'),
                                           (current_timestamp,9,      'twitter'),
                                           (current_timestamp,10,     'twitter')
ON CONFLICT (kyc_step, social)
DO NOTHING;

ALTER TABLE socials DROP CONSTRAINT socials_pkey, ADD PRIMARY KEY (social, user_handle);
ALTER TABLE socials DROP CONSTRAINT IF EXISTS socials_social_user_handle_key;

CREATE INDEX IF NOT EXISTS socials_lookup_userid_idx ON socials (user_id);
CREATE INDEX IF NOT EXISTS socials_lookup_userid_per_social ON socials (user_id, social);
