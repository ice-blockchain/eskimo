-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS social_kyc_unsuccessful_attempts  (
                    created_at                timestamp NOT NULL,
                    kyc_step                  smallint  NOT NULL CHECK (kyc_step = 3 OR kyc_step = 5),
                    reason                    text      NOT NULL,
                    user_id                   text      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    social                    text      NOT NULL CHECK (social = 'twitter' OR social = 'facebook'),
                    PRIMARY KEY (user_id, kyc_step, created_at));

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