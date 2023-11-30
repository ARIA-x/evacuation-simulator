module aria_person.go

go 1.16

require (
	aria_module_person v0.0.0
	github.com/MasterOfBinary/go-opencl v0.0.0-20161217130610-e11c0e14990e // indirect
)

replace aria_module_person => ../../module/person_gpu

replace aria_utility_mqtt => ../../utility/mqtt

replace aria_utility_nodes => ../../utility/nodes

replace aria_utility_settings => ../../utility/settings

replace aria_utility_floods => ../../utility/floods
