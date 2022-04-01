-- SPDX-License-Identifier: BUSL-1.1
box.execute([[CREATE TABLE IF NOT EXISTS users  (
                    id STRING primary key,
                    referredBy STRING REFERENCES users(id) ON DELETE SET NULL,
                    username STRING NOT NULL UNIQUE,
                    email STRING,
                    full_name STRING,
                    phone_number STRING,
                    profile_picture STRING NOT NULL,
                    created_at UNSIGNED NOT NULL
                    updated_at UNSIGNED NOT NULL
                     ) WITH ENGINE = 'vinyl';]])
-- TODO will add indexes later on