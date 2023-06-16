CREATE INDEX view_transactions_tx_type_btree_index ON view_transactions 
    USING btree (tx_type);