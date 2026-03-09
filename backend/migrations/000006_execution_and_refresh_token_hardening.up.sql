UPDATE refresh_tokens
SET token = token_hash
WHERE token IS NOT NULL
  AND token_hash IS NOT NULL
  AND token <> token_hash;

ALTER TABLE executions
ALTER COLUMN project_id DROP NOT NULL;
