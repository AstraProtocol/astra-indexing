package infrastructure

import "time"

const TIME_CACHE_FAST = 5 * time.Second
const TIME_CACHE_MEDIUM = 30 * time.Second
const TIME_CACHE_LONG = 10 * time.Minute

const KAFKA_NEW_DATA_MAX_WAIT = time.Millisecond * 100
const KAFKA_TIME_OUT = time.Second * 5
const KAFKA_FIRST_OFFSET = 0
