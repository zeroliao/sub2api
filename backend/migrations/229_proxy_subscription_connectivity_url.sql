ALTER TABLE proxy_subscription_sources
  ADD COLUMN IF NOT EXISTS connectivity_url TEXT DEFAULT 'https://api.openai.com';
