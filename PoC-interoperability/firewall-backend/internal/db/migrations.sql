create extension if not exists pgcrypto;

create table if not exists users (
  id uuid primary key,
  username text not null unique,
  password_hash text not null,
  role text not null default 'admin',
  created_at timestamptz not null default now()
);

create table if not exists refresh_tokens (
  id uuid primary key,
  user_id uuid not null references users(id) on delete cascade,
  token_hash text not null,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  created_at timestamptz not null default now()
);
create index if not exists idx_refresh_tokens_hash on refresh_tokens(token_hash);

create table if not exists audit_events (
  id uuid primary key,
  actor_id text not null,
  action text not null,
  target text not null,
  payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);
