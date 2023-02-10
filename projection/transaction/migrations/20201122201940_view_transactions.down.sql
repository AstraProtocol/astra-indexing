ALTER TABLE view_transactions DROP CONSTRAINT IF EXISTS transactions_hash_unique;
DROP TABLE IF EXISTS view_transactions;
