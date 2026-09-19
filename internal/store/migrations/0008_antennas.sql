CREATE TABLE antennas (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name       text NOT NULL,
  gain_dbi   double precision NOT NULL,
  freq_mhz   double precision,
  pattern_h  double precision[] NOT NULL CHECK (array_length(pattern_h, 1) = 360),
  source     text,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE links
  ADD COLUMN tx_antenna_id  bigint REFERENCES antennas(id) ON DELETE SET NULL,
  ADD COLUMN tx_azimuth_deg double precision CHECK (tx_azimuth_deg >= 0 AND tx_azimuth_deg < 360),
  ADD COLUMN rx_antenna_id  bigint REFERENCES antennas(id) ON DELETE SET NULL,
  ADD COLUMN rx_azimuth_deg double precision CHECK (rx_azimuth_deg >= 0 AND rx_azimuth_deg < 360);

ALTER TABLE coverages
  ADD COLUMN tx_antenna_id  bigint REFERENCES antennas(id) ON DELETE SET NULL,
  ADD COLUMN tx_azimuth_deg double precision NOT NULL DEFAULT 0 CHECK (tx_azimuth_deg >= 0 AND tx_azimuth_deg < 360);
