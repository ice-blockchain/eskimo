-- SPDX-License-Identifier: BUSL-1.1
box.execute([[CREATE TABLE IF NOT EXISTS users  (
                    id STRING primary key,
                    referred_by STRING REFERENCES users(id) ON DELETE SET NULL,
                    username STRING NOT NULL UNIQUE,
                    email STRING,
                    full_name STRING,
                    phone_number STRING,
                    phone_number_hash STRING,
                    agenda_phone_number_hashes STRING,
                    profile_picture_name STRING NOT NULL,
                    country STRING NOT NULL,
                    hash_code UNSIGNED NOT NULL UNIQUE,
                    created_at UNSIGNED NOT NULL,
                    updated_at UNSIGNED NOT NULL
                     ) WITH ENGINE = 'vinyl';]])
box.execute([[CREATE TABLE IF NOT EXISTS phone_number_validation_codes  (
                    user_id STRING primary key REFERENCES users(id) ON DELETE CASCADE,
                    phone_number STRING NOT NULL UNIQUE,
                    phone_number_hash STRING NOT NULL UNIQUE,
                    validation_code STRING NOT NULL UNIQUE,
                    created_at UNSIGNED NOT NULL
                     ) WITH ENGINE = 'vinyl';]])
box.execute([[CREATE TABLE IF NOT EXISTS users_per_country  (
                    country STRING primary key,
                    user_count UNSIGNED NOT NULL DEFAULT 0
                     ) WITH ENGINE = 'vinyl';]])
box.execute([[CREATE INDEX IF NOT EXISTS users_per_country_user_count_ix ON users_per_country (user_count);]])
box.execute([[CREATE INDEX IF NOT EXISTS users_referral_acquisition_history_ix ON users (referred_by, created_at);]])
box.execute([[CREATE TABLE IF NOT EXISTS days (day UNSIGNED primary key) WITH ENGINE = 'vinyl';]])
box.execute([[INSERT INTO DAYS (DAY) VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9),(10),(11),(12),(13),(14),(15),
                                           (16),(17),(18),(19),(20),(21),(22),(23),(24),(25),(26),(27),(28),(29),(30)]])
-- TODO will add indexes later on
