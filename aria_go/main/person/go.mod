module aria_person.go

go 1.16

require (
	aria_module_person v0.0.0
)

replace aria_module_person => ../../module/person
replace aria_utility_mqtt => ../../utility/mqtt
replace aria_utility_nodes => ../../utility/nodes
replace aria_utility_settings => ../../utility/settings
replace aria_utility_floods => ../../utility/floods

