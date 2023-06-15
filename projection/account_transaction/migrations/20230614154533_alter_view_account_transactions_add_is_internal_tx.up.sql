ALTER TABLE view_account_transactions
    ADD COLUMN "is_internal_tx" BOOLEAN NOT NULL DEFAULT FALSE;