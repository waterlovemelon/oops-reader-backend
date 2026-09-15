-- Reading progress is keyed by the client's book identifier.
-- Date: 2026-09-14
--
-- reading_progress was defined with book_id BIGINT referencing books(id), but
-- `books` is the normalized metadata table and is empty: app-imported books are
-- identified by millisecond ids and catalog books by catalog_books.book_key, so
-- the foreign key rejected every client write. Progress is now keyed by
-- book_key, matching the identity columns of user_catalog_bookshelves.

ALTER TABLE `reading_progress`
    DROP FOREIGN KEY `fk_reading_progress_book_id`;

ALTER TABLE `reading_progress`
    DROP INDEX `fk_reading_progress_book_id`,
    DROP INDEX `uk_user_book`,
    DROP COLUMN `book_id`,
    ADD COLUMN `book_key` VARCHAR(191) NOT NULL DEFAULT '' COMMENT 'Client book identifier: local book id or catalog book key' AFTER `user_id`,
    ADD UNIQUE KEY `uk_user_book_key` (`user_id`, `book_key`);
