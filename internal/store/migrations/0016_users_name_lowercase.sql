UPDATE users SET name = lower(name) WHERE name <> lower(name);
DROP INDEX users_name_lower_idx;
ALTER TABLE users ADD CONSTRAINT users_name_lower CHECK (name = lower(name));
