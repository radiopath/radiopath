-- share_token is stored in the clear: it is the capability for this one row and the owner must see the URL again
ALTER TABLE links     ADD COLUMN share_token text, ADD COLUMN share_expires_at timestamptz;
ALTER TABLE coverages ADD COLUMN share_token text, ADD COLUMN share_expires_at timestamptz;
CREATE UNIQUE INDEX links_share_token_idx     ON links (share_token);
CREATE UNIQUE INDEX coverages_share_token_idx ON coverages (share_token);
