CREATE INDEX view_account_transactions_to_address_btree_index ON view_account_transactions
    USING btree (to_address);