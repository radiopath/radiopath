ALTER TABLE sites     ADD COLUMN owner_id bigint REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE links     ADD COLUMN owner_id bigint REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE coverages ADD COLUMN owner_id bigint REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE antennas  ADD COLUMN owner_id bigint REFERENCES users(id) ON DELETE CASCADE;

UPDATE sites     SET owner_id = COALESCE((SELECT id FROM users WHERE name = created_by), (SELECT min(id) FROM users));
UPDATE links     SET owner_id = COALESCE((SELECT id FROM users WHERE name = created_by), (SELECT min(id) FROM users));
UPDATE coverages SET owner_id = COALESCE((SELECT id FROM users WHERE name = created_by), (SELECT min(id) FROM users));
UPDATE antennas  SET owner_id = COALESCE((SELECT id FROM users WHERE name = created_by), (SELECT min(id) FROM users));
UPDATE links SET tx_antenna_id = NULL WHERE tx_antenna_id IN (SELECT a.id FROM antennas a WHERE a.owner_id <> links.owner_id);
UPDATE links SET rx_antenna_id = NULL WHERE rx_antenna_id IN (SELECT a.id FROM antennas a WHERE a.owner_id <> links.owner_id);
UPDATE coverages SET tx_antenna_id = NULL WHERE tx_antenna_id IN (SELECT a.id FROM antennas a WHERE a.owner_id <> coverages.owner_id);

ALTER TABLE sites     ALTER COLUMN owner_id SET NOT NULL, DROP COLUMN created_by;
ALTER TABLE links     ALTER COLUMN owner_id SET NOT NULL, DROP COLUMN created_by;
ALTER TABLE coverages ALTER COLUMN owner_id SET NOT NULL, DROP COLUMN created_by;
ALTER TABLE antennas  ALTER COLUMN owner_id SET NOT NULL, DROP COLUMN created_by;

CREATE INDEX sites_owner_idx     ON sites (owner_id);
CREATE INDEX links_owner_idx     ON links (owner_id);
CREATE INDEX coverages_owner_idx ON coverages (owner_id);
CREATE INDEX antennas_owner_idx  ON antennas (owner_id);
