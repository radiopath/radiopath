ALTER TABLE antennas
  ADD COLUMN pattern_v double precision[]
    CHECK (pattern_v IS NULL OR coalesce(array_length(pattern_v, 1), 0) = 360);
ALTER TABLE links
  ADD COLUMN tx_tilt_deg double precision NOT NULL DEFAULT 0 CHECK (tx_tilt_deg BETWEEN -90 AND 90),
  ADD COLUMN rx_tilt_deg double precision NOT NULL DEFAULT 0 CHECK (rx_tilt_deg BETWEEN -90 AND 90);

ALTER TABLE coverages
  ADD COLUMN tx_tilt_deg double precision NOT NULL DEFAULT 0 CHECK (tx_tilt_deg BETWEEN -90 AND 90);
