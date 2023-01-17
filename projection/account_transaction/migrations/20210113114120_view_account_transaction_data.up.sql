CREATE TABLE view_account_transaction_data (
   id BIGSERIAL,
   block_height BIGINT,
   block_hash VARCHAR NOT NULL,
   block_time BIGINT NOT NULL,
   hash VARCHAR NOT NULL,
   index INT NOT NULL,
   success BOOLEAN NOT NULL,
   code INT NOT NULL,
   log VARCHAR NOT NULL,
   fee JSONB NOT NULL,
   fee_payer VARCHAR NOT NULL,
   fee_granter VARCHAR NOT NULL,
   gas_wanted BIGINT NOT NULL,
   gas_used BIGINT NOT NULL,
   memo VARCHAR NOT NULL,
   timeout_height BIGINT NOT NULL,
   messages JSONB NOT NULL,
   PRIMARY KEY (id)
)
ALTER TABLE view_account_transaction_data ADD CONSTRAINT hash_unique_account_transaction_data UNIQUE (hash);