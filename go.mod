module github.com/SimonRichardson/juju-api-example

go 1.16

require (
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/go-macaroon-bakery/macaroon-bakery/v3 v3.0.0-20210309064400-d73aa8f92aa2
	github.com/juju/charm/v8 v8.0.0-20211025140802-752458745e56
	github.com/juju/clock v0.0.0-20190205081909-9c5c9712527c
	github.com/juju/cmd v0.0.0-20200108104440-8e43f3faa5c9
	github.com/juju/errors v0.0.0-20210818161939-5560c4c073ff
	github.com/juju/gnuflag v0.0.0-20171113085948-2ce1bb71843d
	github.com/juju/idmclient/v2 v2.0.0-20210309081103-6b4a5212f851
	github.com/juju/juju v0.0.0-00010101000000-000000000000
	github.com/juju/loggo v0.0.0-20210728185423-eebad3a902c4
	github.com/juju/names/v4 v4.0.0-20200929085019-be23e191fee0
	github.com/juju/systems v0.0.0-20200925032749-8c613192c759
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	gopkg.in/juju/charm.v6 v6.0.0-20190729113111-40ffcf7d10e5
	gopkg.in/juju/environschema.v1 v1.0.1-0.20201027142642-c89a4490670a
	gopkg.in/macaroon.v1 v1.0.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/hashicorp/raft => github.com/juju/raft v2.0.0-20200420012049-88ad3b3f0a54+incompatible

replace github.com/juju/juju => /home/simon/go/src/github.com/juju/juju
