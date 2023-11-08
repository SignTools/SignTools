module SignTools

go 1.18

require (
	github.com/ViRb3/koanf-extra v0.0.0-20210725213601-654e724986c4
	github.com/ViRb3/sling/v2 v2.0.2
	github.com/elliotchance/orderedmap v1.5.1
	github.com/eventials/go-tus v0.0.0-20220610120217-05d0564bb571
	github.com/google/go-github/v33 v33.0.0
	github.com/google/uuid v1.4.0
	github.com/knadh/koanf v1.5.0
	github.com/labstack/echo/v4 v4.11.3
	github.com/labstack/gommon v0.4.0
	github.com/natefinch/atomic v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.31.0
	github.com/stretchr/testify v1.8.4
	github.com/tus/tusd v1.13.0
	github.com/ziflex/lecho/v2 v2.5.2
	go.uber.org/atomic v1.11.0
	golang.org/x/crypto v0.14.0
	golang.org/x/oauth2 v0.13.0
	gopkg.in/yaml.v3 v3.0.1
	software.sslmate.com/src/go-pkcs12 v0.4.0
)

replace (
	github.com/tus/tusd v1.13.0 => github.com/SignTools/tusd v0.0.0-20231108184537-0a8f5f25343e
	software.sslmate.com/src/go-pkcs12 v0.4.0 => github.com/SignTools/go-pkcs12 v0.0.0-20231108183519-35ebb50631fc
)

require (
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.4.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
