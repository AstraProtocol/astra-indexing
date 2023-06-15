CREATE INDEX view_account_transaction_data_to_address_btree_index ON view_account_transaction_data
    USING btree (to_address);