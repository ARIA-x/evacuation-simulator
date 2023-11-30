module aria_management.go

go 1.16

require (
	aria_module_media v0.0.0
	aria_module_person v0.0.0
	aria_module_potential v0.0.0
	aria_module_routing v0.0.0
	aria_module_universe v0.0.0
	aria_utility_floods v0.0.0
	aria_utility_mqtt v0.0.0 // indirect
	aria_utility_nodes v0.0.0
	aria_utility_settings v0.0.0
	github.com/MasterOfBinary/go-opencl v0.0.0-20161217130610-e11c0e14990e // indirect
	github.com/eclipse/paho.mqtt.golang v1.3.5 // indirect
	github.com/rs/xid v1.3.0 // indirect
)

replace aria_module_universe => ../../module/universe
replace aria_module_person => ../../module/person_gpu
replace aria_module_potential => ../../module/potential_gpu
replace aria_module_media => ../../module/media
replace aria_module_routing => ../../module/routing
replace aria_utility_mqtt => ../../utility/mqtt
replace aria_utility_nodes => ../../utility/nodes
replace aria_utility_floods => ../../utility/floods
replace aria_utility_settings => ../../utility/settings
