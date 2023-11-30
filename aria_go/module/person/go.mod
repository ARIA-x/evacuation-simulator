module aria_module_person.go

go 1.16

require (
	aria_utility_mqtt v0.0.0
	aria_utility_nodes v0.0.0
	aria_utility_floods v0.0.0
	aria_utility_settings v0.0.0
	github.com/eclipse/paho.mqtt.golang v1.3.4
	github.com/rs/xid v1.3.0
)

replace aria_utility_mqtt => ../../utility/mqtt
replace aria_utility_nodes => ../../utility/nodes
replace aria_utility_floods => ../../utility/floods
replace aria_utility_settings => ../../utility/settings

