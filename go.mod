module SignTools

go 1.17

require (
	github.com/ViRb3/koanf-extra v0.0.0-20210725213601-654e724986c4
	github.com/ViRb3/sling/v2 v2.0.2
	github.com/elliotchance/orderedmap v1.4.0
	github.com/eventials/go-tus v0.0.0-20211022131811-252c8454f2dc
	github.com/google/go-github/v33 v33.0.0
	github.com/google/uuid v1.3.0
	github.com/knadh/koanf v1.4.0
	github.com/labstack/echo/v4 v4.6.1
	github.com/labstack/gommon v0.3.1
	github.com/natefinch/atomic v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.26.0
	github.com/stretchr/testify v1.7.0
	github.com/tus/tusd v1.8.0
	github.com/ziflex/lecho/v2 v2.5.2
	go.uber.org/atomic v1.9.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/oauth2 v0.0.0-20211005180243-6b3c2da341f1
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	software.sslmate.com/src/go-pkcs12 v0.0.0-20210415151418-c5206de65a78
)

replace (
	github.com/eventials/go-tus v0.0.0-20211022131811-252c8454f2dc => github.com/SignTools/go-tus v0.0.0-20211211225219-4d64b8c43f3b
	github.com/tus/tusd v1.8.0 => github.com/SignTools/tusd v1.8.1-0.20211205181817-97252a9e2fa6
	software.sslmate.com/src/go-pkcs12 v0.0.0-20210415151418-c5206de65a78 => github.com/SignTools/go-pkcs12 v0.0.0-20211205185039-6ccc4e597cc9
)

require (
	github.com/bmizerany/pat v0.0.0-20170815010413-6226ea591a40 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/mattn/go-colorable v0.1.11 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.1 // indirect
	golang.org/x/net v0.0.0-20210913180222-943fd674d43e // indirect
	golang.org/x/sys v0.0.0-20211103235746-7861aae1554b // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)
