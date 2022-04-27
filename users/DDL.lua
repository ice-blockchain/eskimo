-- SPDX-License-Identifier: BUSL-1.1
box.execute([[CREATE TABLE IF NOT EXISTS users  (
                    id STRING primary key,
                    hash_code UNSIGNED NOT NULL UNIQUE,
                    referred_by STRING REFERENCES users(id) ON DELETE SET NULL,
                    username STRING NOT NULL UNIQUE,
                    email STRING,
                    full_name STRING,
                    phone_number STRING,
                    profile_picture_name STRING NOT NULL,
                    country STRING NOT NULL,
                    created_at UNSIGNED NOT NULL,
                    updated_at UNSIGNED NOT NULL
                     ) WITH ENGINE = 'vinyl';]])
box.execute([[CREATE TABLE IF NOT EXISTS phone_number_validation_codes  (
                    id STRING primary key,
                    user_id STRING NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    phone_number STRING NOT NULL UNIQUE,
                    validation_code STRING NOT NULL UNIQUE,
                    created_at UNSIGNED NOT NULL
                     ) WITH ENGINE = 'vinyl';]])
-- TODO will add indexes later on
box.execute([[CREATE INDEX IF NOT EXISTS users_username_lookup_ix ON users (username);]])