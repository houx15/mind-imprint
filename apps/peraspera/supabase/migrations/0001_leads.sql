create table if not exists public.leads (
  id uuid primary key default gen_random_uuid(),
  created_at timestamptz not null default now(),
  contact_name text,
  contact text not null,
  child_age text,
  interest text,
  message text,
  locale text,
  source text,
  user_agent text
);
alter table public.leads enable row level security;
-- No public policies: inserts happen server-side with the service key (bypasses RLS).
