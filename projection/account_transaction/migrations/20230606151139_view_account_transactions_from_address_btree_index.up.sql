CREATE INDEX view_account_transactions_from_address_btree_index ON view_account_transactions
    USING btree (from_address);