CREATE INDEX view_account_transaction_data_from_address_btree_index ON view_account_transaction_data
    USING btree (from_address);