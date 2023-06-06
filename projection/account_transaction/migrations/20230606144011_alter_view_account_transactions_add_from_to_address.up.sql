ALTER TABLE view_account_transactions
    ADD COLUMN from_address VARCHAR NOT NULL DEFAULT '',
    ADD COLUMN to_address VARCHAR NOT NULL DEFAULT '';