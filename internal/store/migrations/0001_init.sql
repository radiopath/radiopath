CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE sites (
  id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name             text NOT NULL,
  position         geography(Point,4326) NOT NULL,
  antenna_height_m double precision NOT NULL CHECK (antenna_height_m BETWEEN 0.5 AND 3000),
  created_by       text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sites_position_gix ON sites USING GIST (position);

CREATE TABLE links (
  id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name               text NOT NULL,
  site_a_id          bigint NOT NULL REFERENCES sites(id) ON DELETE RESTRICT,
  site_b_id          bigint NOT NULL REFERENCES sites(id) ON DELETE RESTRICT,
  frequency_mhz      double precision NOT NULL CHECK (frequency_mhz BETWEEN 20 AND 20000),
  tx_power_dbm       double precision NOT NULL,
  tx_gain_dbi        double precision NOT NULL DEFAULT 0,
  rx_gain_dbi        double precision NOT NULL DEFAULT 0,
  tx_line_loss_db    double precision NOT NULL DEFAULT 0 CHECK (tx_line_loss_db >= 0),
  rx_line_loss_db    double precision NOT NULL DEFAULT 0 CHECK (rx_line_loss_db >= 0),
  rx_sensitivity_dbm double precision NOT NULL,
  polarization       smallint NOT NULL DEFAULT 1 CHECK (polarization IN (0,1)),
  created_by         text,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  CHECK (site_a_id <> site_b_id)
);
