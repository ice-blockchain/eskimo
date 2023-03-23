-- SPDX-License-Identifier: ice License 1.0
box.execute([[CREATE TABLE IF NOT EXISTS users  (
                    created_at UNSIGNED NOT NULL,
                    updated_at UNSIGNED NOT NULL,
                    last_mining_started_at UNSIGNED,
                    last_mining_ended_at UNSIGNED,
                    last_ping_cooldown_ended_at UNSIGNED,
                    hidden_profile_elements STRING,
                    random_referred_by BOOLEAN NOT NULL DEFAULT FALSE,
                    verified BOOLEAN NOT NULL DEFAULT FALSE,
                    client_data STRING,
                    phone_number STRING NOT NULL UNIQUE,
                    email STRING NOT NULL UNIQUE,
                    first_name STRING,
                    last_name STRING,
                    country STRING NOT NULL,
                    city STRING NOT NULL,
                    id STRING primary key,
                    username STRING NOT NULL UNIQUE,
                    profile_picture_name STRING NOT NULL,
                    referred_by STRING NOT NULL REFERENCES users(id),
                    phone_number_hash STRING NOT NULL UNIQUE,
                    agenda_phone_number_hashes STRING,
                    mining_blockchain_account_address STRING NOT NULL UNIQUE,
                    blockchain_account_address STRING NOT NULL UNIQUE,
                    language STRING NOT NULL DEFAULT 'en',
                    hash_code UNSIGNED NOT NULL UNIQUE
                     ) WITH ENGINE = 'memtx';]])
box.execute([[INSERT INTO users (created_at,updated_at,phone_number,phone_number_hash,email,id,username,profile_picture_name,referred_by,hash_code,city,country,mining_blockchain_account_address,blockchain_account_address)
                         VALUES (0,0,'bogus','bogus','bogus','bogus','bogus','bogus.jpg','bogus',0,'bogus','RO','bogus','bogus'),
                                (0,0,'icenetwork','icenetwork','icenetwork','icenetwork','icenetwork','icenetwork.jpg','icenetwork',1,'icenetwork','RO','icenetwork','icenetwork');]])
box.execute([[CREATE INDEX IF NOT EXISTS users_referred_by_ix ON users (referred_by);]])
box.execute([[CREATE INDEX IF NOT EXISTS users_lookup_ix ON users (username,first_name,last_name);]])
box.execute([[CREATE TABLE IF NOT EXISTS users_per_country  (
                    country STRING primary key,
                    user_count UNSIGNED NOT NULL DEFAULT 0
                     ) WITH ENGINE = 'memtx';]])
box.execute([[CREATE INDEX IF NOT EXISTS users_per_country_user_count_ix ON users_per_country (user_count);]])
box.execute([[CREATE INDEX IF NOT EXISTS users_referral_acquisition_history_ix ON users (referred_by, created_at);]])
box.execute([[CREATE TABLE IF NOT EXISTS days (day UNSIGNED primary key) WITH ENGINE = 'memtx';]])
box.execute([[INSERT INTO DAYS (DAY) VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9),(10),(11),(12),(13),(14),(15),
                                           (16),(17),(18),(19),(20),(21),(22),(23),(24),(25),(26),(27),(28),(29),(30)]])
-- from [country_short,elevation] -inclusive at both ends- we have ip2location information,
-- everything else (except user_id and updated_at) is from https://github.com/react-native-device-info/react-native-device-info#api
box.execute([[CREATE TABLE IF NOT EXISTS device_metadata  (
                    updated_at              UNSIGNED NOT NULL,
                    first_install_time      UNSIGNED,
                    last_update_time        UNSIGNED,
                    user_id                 STRING NOT NULL REFERENCES users(id),
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
                    device_timezone         STRING,

                    country_short           STRING,
                    country_long            STRING,
                    region                  STRING,
                    city                    STRING,
                    isp                     STRING,
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
                    usage_type              STRING,
                    latitude                NUMBER,
                    longitude               NUMBER,
                    elevation               NUMBER,

                    api_level               UNSIGNED,
                    tablet                  BOOLEAN,
                    pin_or_fingerprint_set  BOOLEAN,
                    emulator                BOOLEAN,
                    primary key(user_id, device_unique_id)) WITH ENGINE = 'memtx';]])
box.execute([[CREATE TABLE IF NOT EXISTS global  (
                    key STRING primary key,
                    value SCALAR NOT NULL
                    ) WITH ENGINE = 'memtx';]])
box.execute([[INSERT INTO global (key,value) VALUES ('TOTAL_USERS', CAST(0 AS UNSIGNED))]])