module vhtime

go 1.23.0

toolchain go1.24.4

require (
	github.com/dkolbly/wl v0.0.0-20180220001605-b06f57e7e2e6
	github.com/godbus/dbus/v5 v5.1.0
	golang.org/x/net v0.38.0
	vhtime/bamboo-core v0.0.0
	vhtime/goibus v0.0.0
)

replace (
	vhtime/bamboo-core => ./bamboo-core
	vhtime/goibus => ./goibus
)
