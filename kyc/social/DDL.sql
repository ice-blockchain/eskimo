-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS social_kyc_unsuccessful_attempts  (
                    created_at                timestamp NOT NULL,
                    kyc_step                  smallint  NOT NULL CHECK (kyc_step = 3 OR kyc_step = 5),
                    reason                    text      NOT NULL,
                    user_id                   text      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    PRIMARY KEY (user_id, kyc_step, created_at));

CREATE INDEX IF NOT EXISTS social_kyc_unsuccessful_attempts_lookup1_ix ON social_kyc_unsuccessful_attempts (kyc_step,social,created_at DESC);
CREATE INDEX IF NOT EXISTS social_kyc_unsuccessful_attempts_lookup2_ix ON social_kyc_unsuccessful_attempts (kyc_step,social,created_at DESC,(CASE
                                                                                                                                                WHEN reason like 'duplicate userhandle %' THEN 'duplicate userhandle'
                                                                                                                                                WHEN reason like '%: %' THEN substring(reason from position(': ' in reason) + 2)
                                                                                                                                                ELSE reason
                                                                                                                                            END));

CREATE TABLE IF NOT EXISTS social_kyc_steps (
                    created_at                timestamp NOT NULL,
                    kyc_step                  smallint  NOT NULL CHECK (kyc_step = 3 OR kyc_step = 5),
                    user_id                   text      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    user_handle               text      NOT NULL,
                    PRIMARY KEY (user_id, kyc_step));

CREATE TABLE IF NOT EXISTS socials (
                    user_id                   text      NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    user_handle               text      NOT NULL,
                    UNIQUE (social, user_handle));

CREATE TABLE IF NOT EXISTS unsuccessful_social_kyc_alerts (
                    last_alert_at             timestamp NOT NULL,
                    kyc_step                  smallint  NOT NULL CHECK (kyc_step = 3 OR kyc_step = 5),
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    PRIMARY KEY (kyc_step, social))
                    WITH (FILLFACTOR = 70);

insert into unsuccessful_social_kyc_alerts (last_alert_at,    kyc_step,social)
                                    VALUES (current_timestamp,3,      'twitter'),
                                           (current_timestamp,5,      'twitter')
ON CONFLICT (kyc_step, social)
DO NOTHING;