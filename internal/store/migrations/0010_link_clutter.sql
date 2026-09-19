ALTER TABLE links ADD COLUMN clutter_loss_db double precision NOT NULL DEFAULT 0
  CHECK (clutter_loss_db >= 0);
