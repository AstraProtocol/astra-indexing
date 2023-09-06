ALTER TABLE view_account_transactions
    ADD CONSTRAINT account_transactions_unique
    UNIQUE (block_height, account, transaction_hash, success, from_address, to_address, is_internal_tx, tx_index);