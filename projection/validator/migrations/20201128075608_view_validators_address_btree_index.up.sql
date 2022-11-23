CREATE INDEX view_validators_operator_address_btree_index ON view_validators USING btree (operator_address);
CREATE INDEX view_validators_consensus_node_address_btree_index ON view_validators USING btree (consensus_node_address);
