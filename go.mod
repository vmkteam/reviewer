module reviewsrv

go 1.25

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/brianvoe/gofakeit/v7 v7.14.0
	github.com/getsentry/sentry-go v0.41.0
	github.com/go-pg/pg/v10 v10.15.0
	github.com/go-pg/urlstruct v1.0.1
	github.com/go-playground/validator/v10 v10.30.1
	github.com/google/uuid v1.6.0
	github.com/hypnoglow/go-pg-monitor v1.2.0
	github.com/hypnoglow/go-pg-monitor/gopgv10 v1.2.0
	github.com/labstack/echo/v4 v4.15.0
	github.com/namsral/flag v1.7.4-pre
	github.com/prometheus/client_golang v1.23.2
	github.com/smartystreets/goconvey v1.8.1
	github.com/stretchr/testify v1.11.1
	github.com/vmkteam/appkit v0.1.2
	github.com/vmkteam/embedlog v0.1.3
	github.com/vmkteam/rpcgen/v2 v2.5.4
	github.com/vmkteam/zenrpc-middleware v1.3.2
	github.com/vmkteam/zenrpc/v2 v2.3.1
	golang.org/x/crypto v0.47.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/codemodus/kace v0.5.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/getsentry/sentry-go/echo v0.41.0 // indirect
	github.com/go-pg/zerochecker v0.2.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/iancoleman/orderedmap v0.3.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lmittmann/tint v1.1.2 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/smarty/assertions v1.16.0 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/vmihailenco/bufpool v0.1.11 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser v0.1.2 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/vmkteam/meta-schema/v2 v2.0.1 // indirect
	github.com/vmkteam/zenrpc v1.1.1 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mellium.im/sasl v0.3.2 // indirect
)

tool github.com/vmkteam/zenrpc/v2/zenrpc

replace github.com/go-pg/pg/v10 => github.com/vmkteam/pg/v10 v10.15.0-custom
