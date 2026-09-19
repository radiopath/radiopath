ALTER TABLE coverages
  ADD COLUMN legend_db     double precision[] CHECK (array_length(legend_db, 1) BETWEEN 1 AND 8),
  ADD COLUMN legend_colors text[] CHECK (array_length(legend_colors, 1) = array_length(legend_db, 1));
