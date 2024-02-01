-- SPDX-License-Identifier: ice License 1.0

create table if not exists questions
(
    id             bigint    not null,
    correct_option smallint  not null,
    options        text[]    not null,
    language       text      not null,
    question       text      not null,
    unique (question, language),
    primary key (language, id)
);

create table if not exists failed_quiz_sessions
(
    started_at         timestamp  not null,
    ended_at           timestamp  not null,
    skipped            boolean    not null default false,
    questions          bigint[]   not null,
    answers            smallint[] not null,
    user_id            text       not null references users (id) ON DELETE CASCADE,
    language           text       not null,
    primary key (user_id, started_at)
);

CREATE INDEX IF NOT EXISTS failed_quiz_sessions_lookup1_ix ON failed_quiz_sessions (ended_at DESC);

create table if not exists failed_quiz_sessions_history
(
    created_at         timestamp  not null default current_timestamp,
    started_at         timestamp  not null,
    ended_at           timestamp  not null,
    skipped            boolean    not null default false,
    questions          bigint[]   not null,
    answers            smallint[] not null,
    user_id            text       not null references users (id) ON DELETE CASCADE,
    language           text       not null,
    primary key (user_id, started_at)
);

create table if not exists quiz_sessions
(
    started_at         timestamp  not null,
    ended_at           timestamp,
    ended_successfully boolean    not null default false,
    questions          bigint[]   not null,
    answers            smallint[] not null,
    user_id            text       primary key references users (id) ON DELETE CASCADE,
    language           text       not null
);

CREATE INDEX IF NOT EXISTS quiz_sessions_lookup1_ix ON quiz_sessions (ended_successfully,ended_at DESC NULLS LAST);

CREATE TABLE IF NOT EXISTS quiz_alerts (
                    last_alert_at             timestamp NOT NULL,
                    frequency_in_seconds      bigint    NOT NULL DEFAULT 300,
                    pk                        smallint  NOT NULL PRIMARY KEY)
                    WITH (FILLFACTOR = 70);

insert into quiz_alerts (last_alert_at,     pk)
                 VALUES (current_timestamp, 1)
ON CONFLICT (pk)
DO NOTHING;

CREATE TABLE IF NOT EXISTS quiz_resets (
    user_id text        NOT NULL PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    resets  timestamp[] NOT NULL
) WITH (FILLFACTOR = 70);