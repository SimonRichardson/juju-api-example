module github.com/SimonRichardson/juju-api-example

go 1.16

require (
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/go-macaroon-bakery/macaroon-bakery/v3 v3.0.0-20210309064400-d73aa8f92aa2
	github.com/juju/charm/v8 v8.0.0-20211025140802-752458745e56
	github.com/juju/clock v0.0.0-20190205081909-9c5c9712527c
	github.com/juju/collections v0.0.0-20200605021417-0d0ec82b7271
	github.com/juju/errors v0.0.0-20210818161939-5560c4c073ff
	github.com/juju/idmclient/v2 v2.0.0-20210309081103-6b4a5212f851
	github.com/juju/juju v0.0.0-20211201065255-8a154b7d629f
	github.com/juju/names/v4 v4.0.0-20200929085019-be23e191fee0
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	gopkg.in/juju/environschema.v1 v1.0.1-0.20201027142642-c89a4490670a
)

replace github.com/hashicorp/raft => github.com/juju/raft v2.0.0-20200420012049-88ad3b3f0a54+incompatible
