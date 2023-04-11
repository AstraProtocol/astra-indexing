package infrastructure

import "time"

const TIME_CACHE_FAST = 5 * time.Second
const TIME_CACHE_MEDIUM = 30 * time.Second
const TIME_CACHE_LONG = 10 * time.Minute

const KAFKA_NEW_DATA_MAX_WAIT = time.Millisecond * 350
const KAFKA_TIME_OUT = time.Second * 3
const KAFKA_READ_BATCH_TIME_OUT = time.Second * 60
