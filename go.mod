module SignTools

go 1.26.0

require (
	github.com/ViRb3/koanf-extra v0.0.0-20241224160111-fad8e9827c5f
	github.com/ViRb3/sling/v2 v2.0.2
	github.com/aixiansheng/lzfse v0.2.4
	github.com/elliotchance/orderedmap v1.8.0
	github.com/eventials/go-tus v0.0.0-20250612203642-7827b129cd4c
	github.com/galecore/xslog v0.0.0-20230717081035-da7669fe4648
	github.com/google/go-github/v33 v33.0.0
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf v1.5.0
	github.com/labstack/echo/v4 v4.16.3
	github.com/labstack/gommon v0.5.0
	github.com/natefinch/atomic v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.35.1
	github.com/stretchr/testify v1.12.1
	github.com/tus/tusd/v2 v2.10.1
	github.com/ziflex/lecho/v2 v2.5.2
	go.uber.org/atomic v1.12.0
	golang.org/x/crypto v0.58.0
	golang.org/x/exp v0.0.0-20261109185158-a52e3aef5920
	golang.org/x/oauth2 v0.38.0
	gopkg.in/yaml.v3 v3.0.1
	howett.net/plist v1.0.1
	software.sslmate.com/src/go-pkcs12 v0.7.4
)

replace (
	github.com/tus/tusd/v2 v2.10.1 => github.com/SignTools/tusd/v2 v2.0.0-20261105210929-99d33c50be0f
	software.sslmate.com/src/go-pkcs12 v0.7.4 => github.com/SignTools/go-pkcs12 v0.0.0-20261105210929-5457a6812520
)

require (
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/jba/slog v0.0.0-20230403194657-e1c00ce43c8a // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/tus/lockfile v1.2.0 // indirect
	github.com/tus/tusd v1.13.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	golang.org/x/time v0.16.0 // indirect
)
