CREATE INDEX view_account_transactions_account_is_internal_tx_partial_index ON view_account_transactions (account)
    WHERE is_internal_tx IS FALSE;