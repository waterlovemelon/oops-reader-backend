-- Canonical catalog shelf key.
-- Date: 2026-09-15
--
-- user_catalog_bookshelves.catalog_book_key stored the bare catalog_books.book_key
-- while reading_progress.book_key stores the same book as "catalog:<book_key>".
-- The shelf and the progress projection of "recently read" therefore pointed at
-- different identifiers for the same online book. Both now use the canonical
-- catalog:<id> form; the application normalizes on write from here on.
--
-- Apply with:
--   mariadb -u root -p oops_reader < migrations/015_canonical_catalog_shelf_key.sql

-- A bare row that would collide with an existing canonical row loses on
-- uk_user_catalog_book (user_id, catalog_book_key), so drop it before renaming.
DELETE `bare`
FROM `user_catalog_bookshelves` `bare`
JOIN `user_catalog_bookshelves` `canonical`
    ON `canonical`.`user_id` = `bare`.`user_id`
   AND `canonical`.`catalog_book_key` = CONCAT('catalog:', `bare`.`catalog_book_key`)
WHERE `bare`.`catalog_book_key` NOT LIKE 'catalog:%';

UPDATE `user_catalog_bookshelves`
SET `catalog_book_key` = CONCAT('catalog:', `catalog_book_key`),
    `updated_at` = NOW()
WHERE `catalog_book_key` NOT LIKE 'catalog:%';
