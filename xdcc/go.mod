module github.com/ProgramComputer/plugin-XDCC

replace github.com/kiwiirc/webircgateway => ../webircgateway

go 1.18

require (
	github.com/OneOfOne/xxhash v1.2.8
	github.com/gobwas/glob v0.2.3
	github.com/golang-jwt/jwt/v4 v4.4.2
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.5.0
	github.com/igm/sockjs-go/v3 v3.0.2
	github.com/orcaman/concurrent-map v1.0.0
	golang.org/x/crypto v0.1.0
	golang.org/x/net v0.1.0
	golang.org/x/time v0.1.0
	gopkg.in/ini.v1 v1.67.0
)

require (
	github.com/kiwiirc/webircgateway v0.0.0-20221028163248-bb110ab6c4ca // indirect
	golang.org/x/exp v0.0.0-20221217163422-3c43f8badb15 // indirect
)
