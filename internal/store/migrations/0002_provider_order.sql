ALTER TABLE providers ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
UPDATE providers SET sort_order = id;
PRAGMA user_version = 2;
