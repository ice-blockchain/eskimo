-- SPDX-License-Identifier: BUSL-1.1
box.execute([[CREATE TABLE IF NOT EXISTS users  (
                    created_at UNSIGNED NOT NULL,
                    updated_at UNSIGNED NOT NULL,
                    last_mining_started_at UNSIGNED DEFAULT 0,
                    last_ping_at UNSIGNED DEFAULT 0,
                    id STRING primary key,
                    username STRING NOT NULL UNIQUE,
                    full_name STRING,
                    phone_number STRING,
                    profile_picture_name STRING NOT NULL,
                    country STRING NOT NULL,
                    email STRING,
                    referred_by STRING REFERENCES users(id) ON DELETE SET NULL,
                    phone_number_hash STRING,
                    agenda_phone_number_hashes STRING,
                    hash_code UNSIGNED NOT NULL UNIQUE
                     ) WITH ENGINE = 'vinyl';]])
box.execute([[CREATE INDEX IF NOT EXISTS users_referred_by_ix ON users (referred_by);]])
box.execute([[CREATE INDEX IF NOT EXISTS users_phone_number_hash_ix ON users (phone_number_hash);]])
box.execute([[CREATE INDEX IF NOT EXISTS users_lookup_ix ON users (username,full_name);]])
box.execute([[CREATE TABLE IF NOT EXISTS phone_number_validations  (
                    created_at UNSIGNED NOT NULL,
                    user_id STRING primary key REFERENCES users(id) ON DELETE CASCADE,
                    phone_number STRING NOT NULL UNIQUE,
                    phone_number_hash STRING NOT NULL UNIQUE,
                    validation_code STRING NOT NULL UNIQUE
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
box.execute([[CREATE TABLE IF NOT EXISTS device_settings  (
                    updated_at                  UNSIGNED NOT NULL,
                    push_notification_channels  STRING,
                    user_id                     STRING NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    device_unique_id            STRING NOT NULL,
                    language                    STRING NOT NULL DEFAULT 'en',
                    primary key(user_id, device_unique_id) ) WITH ENGINE = 'vinyl';]])
-- from [country_short,usagetype] -inclusive at both ends- we have ip2location information,
-- everything else (except user_id and updated_at) is from https://github.com/react-native-device-info/react-native-device-info#api
box.execute([[CREATE TABLE IF NOT EXISTS device_metadata  (
                    country_short           STRING,
                    country_long            STRING,
                    region                  STRING,
                    city                    STRING,
                    isp                     STRING,
                    latitude                DOUBLE,
                    longitude               DOUBLE,
                    domain                  STRING,
                    zipcode                 STRING,
                    timezone                STRING,
                    net_speed               STRING,
                    idd_code                STRING,
                    area_code               STRING,
                    weather_station_code    STRING,
                    weather_station_name    STRING,
                    mcc                     STRING,
                    mnc                     STRING,
                    mobile_brand            STRING,
                    elevation               DOUBLE,
                    usage_type              STRING,

                    updated_at              UNSIGNED NOT NULL,
                    first_install_time      UNSIGNED,
                    last_update_time        UNSIGNED,
                    user_id                 STRING NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    device_unique_id        STRING NOT NULL,
                    readable_version        STRING,
                    fingerprint             STRING,
                    instance_id             STRING,
                    hardware                STRING,
                    product                 STRING,
                    device                  STRING,
                    type                    STRING,
                    tags                    STRING,
                    device_id               STRING,
                    device_type             STRING,
                    device_name             STRING,
                    brand                   STRING,
                    carrier                 STRING,
                    manufacturer            STRING,
                    user_agent              STRING,
                    system_name             STRING,
                    system_version          STRING,
                    base_os                 STRING,
                    build_id                STRING,
                    bootloader              STRING,
                    codename                STRING,
                    installer_package_name  STRING,
                    push_notification_token STRING,
                    api_level               UNSIGNED,
                    tablet                  BOOLEAN,
                    pin_or_fingerprint_set  BOOLEAN,
                    emulator                BOOLEAN,
                    primary key(user_id, device_unique_id)) WITH ENGINE = 'vinyl';]])
-- TODO will add indexes later on
