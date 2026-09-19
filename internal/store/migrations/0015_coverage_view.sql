-- view_overlays is narrowed in SQL to the owner's own computed coverages on write and read, so a stale id leaks nothing
ALTER TABLE coverages
  ADD COLUMN view_opacity  smallint,
  ADD COLUMN view_overlays bigint[],
  ADD COLUMN view_circles  boolean;
