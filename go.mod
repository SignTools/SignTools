module SignTools

go 1.22.0
toolchain go1.24.1

require (
	github.com/ViRb3/koanf-extra v0.0.0-20241224160111-fad8e9827c5f
	github.com/ViRb3/sling/v2 v2.0.2
	github.com/elliotchance/orderedmap v1.8.0
	github.com/eventials/go-tus v0.0.0-20220610120217-05d0564bb571
	github.com/galecore/xslog v0.0.0-20230717081035-da7669fe4648
	github.com/google/go-github/v33 v33.0.0
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf v1.5.0
	github.com/labstack/echo/v4 v4.13.3
	github.com/labstack/gommon v0.4.2
	github.com/natefinch/atomic v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.33.0
	github.com/stretchr/testify v1.10.0
	github.com/tus/tusd/v2 v2.6.0
	github.com/ziflex/lecho/v2 v2.5.2
	go.uber.org/atomic v1.11.0
	golang.org/x/crypto v0.37.0
	golang.org/x/exp v0.0.0-20250210185358-939b2ce775ac
	golang.org/x/oauth2 v0.26.0
	gopkg.in/yaml.v3 v3.0.1
	software.sslmate.com/src/go-pkcs12 v0.5.0
)

replace (
	github.com/tus/tusd/v2 v2.6.0 => github.com/SignTools/tusd/v2 v2.0.0-20250213234003-9bbfab9e892b
	software.sslmate.com/src/go-pkcs12 v0.5.0 => github.com/SignTools/go-pkcs12 v0.0.0-20250213234444-4065634ac0c8
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/jba/slog v0.0.0-20230403194657-e1c00ce43c8a // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tus/lockfile v1.2.0 // indirect
	github.com/tus/tusd v1.13.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.9.0 // indirect
)
