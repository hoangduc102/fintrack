# Synchronous Worker-Pool Architecture for CSV Import

We decided to execute CSV transaction imports synchronously within the HTTP request lifecycle using an in-memory worker pool built with standard Go goroutines and channels. A single reader passes rows to worker goroutines for validation and category mapping against an in-memory cache, and a collector persists valid records to PostgreSQL in a transaction with partial failure reporting. 

Processing propagates the request `context.Context` so that client disconnects cleanly terminate workers, drain channels without leaks, and abort the database transaction. Channel ownership is explicit: the producer owns closing `jobs`, the coordinator owns closing `results`, and workers never close shared channels. This satisfies the Go concurrency learning requirement while avoiding external task queues (e.g. Redis, RabbitMQ) or polling mechanisms.
