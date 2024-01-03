.PHONY: e2e 

include .env

e2e:
	# parenthesis preserve current working directory
	(go run ./main.go infra --subscription=${SUBSCRIPTION_ID} --tenant=${TENANT_ID} --names=${INFRA_NAMES} && \
	 go run ./main.go test)


runinfra: 
	go run ./main.go infra --subscription=${SUBSCRIPTION_ID} --tenant=${TENANT_ID} --names=${INFRA_NAMES} 

test:
	go run ./main.go test