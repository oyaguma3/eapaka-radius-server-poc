module github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server

go 1.25.5

require (
	github.com/alicebob/miniredis/v2 v2.36.1
	github.com/go-resty/resty/v2 v2.17.1
	github.com/google/uuid v1.6.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/oyaguma3/go-eapaka v0.0.0-20260222130953-db627581125a
	github.com/redis/go-redis/v9 v9.17.3
	github.com/sony/gobreaker v1.0.0
	go.uber.org/mock v0.6.0
	layeh.com/radius v0.0.0-20231213012653-1006025d24f8
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	golang.org/x/net v0.46.0 // indirect
)
