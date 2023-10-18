package infrastructure

import "time"

const TIME_CACHE_VERY_FAST = 5 * time.Second
const TIME_CACHE_FAST = 10 * time.Second
const TIME_CACHE_MEDIUM = 45 * time.Second
const TIME_CACHE_LONG = 15 * time.Minute

const KAFKA_NEW_DATA_MAX_WAIT = time.Second * 10
const KAFKA_TIME_OUT = time.Second * 3
const KAFKA_READ_BATCH_TIME_OUT = time.Second * 60
const KAFKA_READ_BACKOFF_MAX = time.Second * 1

const CA_CERT_LOCAL_PATH = "./certs/ca.crt"

const CA_CERT_PATH = "/certs/chainindexing.kafka.prod/ca.crt"
const TLS_CERT_PATH = "/certs/chainindexing.kafka.prod/tls.crt"
const TLS_KEY_PATH = "/certs/chainindexing.kafka.prod/tls.key"

const CA_CERT_PATH_DEV = "/certs/chainindexing.kafka.dev/ca.crt"
const TLS_CERT_PATH_DEV = "/certs/chainindexing.kafka.dev/tls.crt"
const TLS_KEY_PATH_DEV = "/certs/chainindexing.kafka.dev/tls.key"

const EVM_TXS_TOPIC = "evm-txs"
const INTERNAL_TXS_TOPIC = "internal-txs"
const TOKEN_TRANSFERS_TOPIC = "token-transfers"
