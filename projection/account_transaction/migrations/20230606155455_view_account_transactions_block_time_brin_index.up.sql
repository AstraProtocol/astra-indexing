CREATE INDEX view_account_transactions_block_time_brin_index ON view_account_transactions
    USING brin (block_time);