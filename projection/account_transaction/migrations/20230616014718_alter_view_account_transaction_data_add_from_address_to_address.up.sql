ALTER TABLE view_account_transaction_data
    ADD COLUMN from_address VARCHAR NOT NULL DEFAULT '',
    ADD COLUMN to_address VARCHAR NOT NULL DEFAULT '';