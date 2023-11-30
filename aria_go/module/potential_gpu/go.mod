module aria_module_potential.go

go 1.16

require (
	aria_utility_mqtt v0.0.0
	github.com/eclipse/paho.mqtt.golang v1.3.4
	github.com/rs/xid v1.3.0
)

replace aria_utility_mqtt => ../../utility/mqtt

