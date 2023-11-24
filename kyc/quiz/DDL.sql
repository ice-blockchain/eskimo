-- SPDX-License-Identifier: ice License 1.0

create table if not exists questions
(
    id             bigint not null generated always as identity,
    correct_option smallint  not null,
    options        text[]    not null,
    language       text      not null,
    question       text      not null,
    unique (question, language),
    primary key (language, id)
);

create table if not exists failed_quizz_sessions
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

create table if not exists quizz_sessions
(
    started_at         timestamp  not null,
    ended_at           timestamp,
    ended_successfully boolean    not null default false,
    questions          bigint[]   not null,
    answers            smallint[] not null,
    user_id            text       primary key references users (id) ON DELETE CASCADE,
    language           text       not null
);
