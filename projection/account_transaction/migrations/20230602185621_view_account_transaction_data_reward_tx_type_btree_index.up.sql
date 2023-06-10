CREATE INDEX view_account_transaction_data_reward_tx_type_btree_index ON view_account_transaction_data 
    USING btree (reward_tx_type);