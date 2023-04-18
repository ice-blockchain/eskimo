-- SPDX-License-Identifier: ice License 1.0
CREATE TABLE IF NOT EXISTS users  (
                    created_at timestamp NOT NULL,
                    updated_at timestamp NOT NULL,
                    last_mining_started_at timestamp,
                    last_mining_ended_at timestamp,
                    last_ping_cooldown_ended_at timestamp,
                    hidden_profile_elements text,
                    random_referred_by BOOLEAN NOT NULL DEFAULT FALSE,
                    verified BOOLEAN NOT NULL DEFAULT FALSE,
                    client_data text,
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
                    agenda_phone_number_hashes text,
                    mining_blockchain_account_address text NOT NULL UNIQUE,
                    blockchain_account_address text NOT NULL UNIQUE,
                    language text NOT NULL DEFAULT 'en',
                    hash_code BIGINT NOT NULL UNIQUE
                     );
INSERT INTO users (created_at,updated_at,phone_number,phone_number_hash,email,id,username,profile_picture_name,referred_by,hash_code,city,country,mining_blockchain_account_address,blockchain_account_address)
                         VALUES (to_timestamp(0),to_timestamp(0),'bogus','bogus','bogus','bogus','bogus','bogus.jpg','bogus',0,'bogus','RO','bogus','bogus'),
                                (to_timestamp(0),to_timestamp(0),'icenetwork','icenetwork','icenetwork','icenetwork','icenetwork','icenetwork.jpg','icenetwork',1,'icenetwork','RO','icenetwork','icenetwork')
ON CONFLICT DO NOTHING;
CREATE INDEX IF NOT EXISTS users_referred_by_ix ON users (referred_by);
CREATE INDEX IF NOT EXISTS users_username_ix ON users (username);
CREATE INDEX IF NOT EXISTS users_lookup_ix ON users (username,first_name,last_name);
CREATE INDEX IF NOT EXISTS users_created_at_ix on users(created_at);

CREATE TABLE IF NOT EXISTS agenda_phone_number_hashes (
      user_id                     TEXT NOT NULL REFERENCES users(id),
      agenda_phone_number_hash    TEXT NOT NULL,
      PRIMARY KEY(user_id, agenda_phone_number_hash)
);

CREATE TABLE IF NOT EXISTS users_per_country  (
                    country text primary key,
                    user_count BIGINT NOT NULL DEFAULT 0
                     );
CREATE INDEX IF NOT EXISTS users_per_country_user_count_ix ON users_per_country (user_count);
CREATE INDEX IF NOT EXISTS users_referral_acquisition_history_ix ON users (referred_by, created_at);
CREATE TABLE IF NOT EXISTS days (day SMALLINT primary key);
INSERT INTO DAYS (DAY) VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9),(10),(11),(12),(13),(14),(15),
                              (16),(17),(18),(19),(20),(21),(22),(23),(24),(25),(26),(27),(28),(29),(30)
ON CONFLICT DO NOTHING;
-- from [country_short,elevation] -inclusive at both ends- we have ip2location information,
-- everything else (except user_id and updated_at) is from https://github.com/react-native-device-info/react-native-device-info#api
CREATE TABLE IF NOT EXISTS device_metadata  (
                    updated_at              timestamp NOT NULL,
                    first_install_time      timestamp,
                    last_update_time        timestamp,
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
                    latitude                NUMERIC,
                    longitude               NUMERIC,
                    elevation               NUMERIC,

                    api_level               SMALLINT,
                    tablet                  BOOLEAN,
                    pin_or_fingerprint_set  BOOLEAN,
                    emulator                BOOLEAN,
                    primary key(user_id, device_unique_id));
CREATE TABLE IF NOT EXISTS global  (
                    key text primary key,
                    value bigint NOT NULL
                    );
INSERT INTO global (key,value) VALUES ('TOTAL_USERS', 0) ON CONFLICT DO NOTHING;