-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS users  (
                    created_at timestamp NOT NULL,
                    updated_at timestamp NOT NULL,
                    last_mining_started_at timestamp,
                    last_mining_ended_at timestamp,
                    last_ping_cooldown_ended_at timestamp,
                    hash_code bigint not null generated always as identity,
                    random_referred_by BOOLEAN NOT NULL DEFAULT FALSE,
                    kyc_passed BOOLEAN NOT NULL DEFAULT FALSE,
                    client_data text,
                    hidden_profile_elements text[],
                    phone_number text NOT NULL UNIQUE,
                    email text NOT NULL UNIQUE,
                    first_name text,
                    last_name text,
                    country text NOT NULL,
                    city text NOT NULL,
                    id text primary key,
                    username text NOT NULL UNIQUE,
                    profile_picture_name text NOT NULL,
                    referred_by text NOT NULL REFERENCES users(id),
                    phone_number_hash text NOT NULL UNIQUE,
                    agenda_contact_user_ids text[],
                    mining_blockchain_account_address text NOT NULL UNIQUE,
                    blockchain_account_address text NOT NULL UNIQUE,
                    language text NOT NULL DEFAULT 'en',
                    lookup tsvector NOT NULL)
                    WITH (FILLFACTOR = 70);
INSERT INTO users (created_at,updated_at,phone_number,phone_number_hash,email,id,username,profile_picture_name,referred_by,city,country,mining_blockchain_account_address,blockchain_account_address, lookup)
                         VALUES (current_timestamp,current_timestamp,'bogus','bogus','bogus','bogus','bogus','bogus.jpg','bogus','bogus','RO','bogus','bogus',to_tsvector('bogus')),
                                (current_timestamp,current_timestamp,'icenetwork','icenetwork','icenetwork','icenetwork','icenetwork','icenetwork.jpg','icenetwork','icenetwork','RO','icenetwork','icenetwork',to_tsvector('icenetwork'))
ON CONFLICT DO NOTHING;
CREATE INDEX IF NOT EXISTS users_referred_by_ix ON users (referred_by);
CREATE EXTENSION IF NOT EXISTS btree_gin;
CREATE INDEX IF NOT EXISTS users_lookup_gin_idx ON users USING GIN (lookup);
CREATE TABLE IF NOT EXISTS users_per_country  (
                    user_count BIGINT NOT NULL DEFAULT 0,
                    country text primary key
                     );


CREATE TABLE IF NOT EXISTS device_metadata  (
                    updated_at              timestamp NOT NULL,
                    first_install_time      timestamp,
                    last_update_time        timestamp,
                    latitude                NUMERIC,
                    longitude               NUMERIC,
                    elevation               NUMERIC,
                    api_level               SMALLINT,
                    tablet                  BOOLEAN,
                    pin_or_fingerprint_set  BOOLEAN,
                    emulator                BOOLEAN,
                    user_id                 text NOT NULL REFERENCES users(id),
                    device_unique_id        text NOT NULL,
                    readable_version        text,
                    fingerprint             text,
                    instance_id             text,
                    hardware                text,
                    product                 text,
                    device                  text,
                    type                    text,
                    tags                    text,
                    device_id               text,
                    device_type             text,
                    device_name             text,
                    brand                   text,
                    carrier                 text,
                    manufacturer            text,
                    user_agent              text,
                    system_name             text,
                    system_version          text,
                    base_os                 text,
                    build_id                text,
                    bootloader              text,
                    codename                text,
                    installer_package_name  text,
                    push_notification_token text,
                    device_timezone         text,
                    country_short           text,
                    country_long            text,
                    region                  text,
                    city                    text,
                    isp                     text,
                    domain                  text,
                    zipcode                 text,
                    timezone                text,
                    net_speed               text,
                    idd_code                text,
                    area_code               text,
                    weather_station_code    text,
                    weather_station_name    text,
                    mcc                     text,
                    mnc                     text,
                    mobile_brand            text,
                    usage_type              text,
                    primary key(user_id, device_unique_id))
                    WITH (FILLFACTOR = 70);
CREATE TABLE IF NOT EXISTS global  (
                    value bigint NOT NULL,
                    key text primary key)
                    WITH (FILLFACTOR = 70);
INSERT INTO global (key,value) VALUES ('TOTAL_USERS', 0) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS referral_acquisition_history (
     T1                      BIGINT DEFAULT 0,
     T1_TODAY                BIGINT DEFAULT 0,
     T1_TODAY_MINUS_1        BIGINT DEFAULT 0,
     T1_TODAY_MINUS_2        BIGINT DEFAULT 0,
     T1_TODAY_MINUS_3        BIGINT DEFAULT 0,
     T1_TODAY_MINUS_4        BIGINT DEFAULT 0,
     T2                      BIGINT DEFAULT 0,
     T2_TODAY                BIGINT DEFAULT 0,
     T2_TODAY_MINUS_1        BIGINT DEFAULT 0,
     T2_TODAY_MINUS_2        BIGINT DEFAULT 0,
     T2_TODAY_MINUS_3        BIGINT DEFAULT 0,
     T2_TODAY_MINUS_4        BIGINT DEFAULT 0,
     DATE                    DATE NOT NULL,
     USER_ID                 TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS processed_referrals (
                            processed_at            TIMESTAMP,
                            user_id                 TEXT,
                            referred_by             TEXT,
                            primary key (user_id, referred_by)
);
CREATE INDEX IF NOT EXISTS processed_referrals_processed_at_ix ON processed_referrals (processed_at);
