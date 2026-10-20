-- Modify "dip" table to allow both status labels.
ALTER TABLE `dip` MODIFY COLUMN `status` enum('queued','in progress','done','stored','failed','deleted') NOT NULL;

-- Update existing completed DIPs to the stored status label.
UPDATE `dip` SET `status` = 'stored' WHERE `status` = 'done';

-- Remove the legacy done status label.
ALTER TABLE `dip` MODIFY COLUMN `status` enum('queued','in progress','stored','failed','deleted') NOT NULL;
