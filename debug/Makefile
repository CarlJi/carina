# set default shell
SHELL=/bin/bash -o pipefail -o errexit

IMAGE_REPOSITORY=registry.cn-hangzhou.aliyuncs.com
TAG ?= debug
PLATFORM ?= linux/arm64,linux/amd64


kversion ?=v1.20.2
 
export NODE_NAME = dev1-master.novalocal

build:
	go build  -o bin/proxyclient node-proxy-client/client.go
	go build  -o bin/proxyserver local-proxy-server/server.go
	chmod a+x    bin/*

image:
	docker buildx build -t $(IMAGE_REPOSITORY)/antmoveh/carina-proxy:$(TAG) --platform=$(PLATFORM) -o type=registry . 
	docker buildx imagetools inspect $(IMAGE_REPOSITORY)/antmoveh/carina-proxy:$(TAG)

clean:
	@rm -rf hack/certs
	docker stop $(shell docker ps -a  |grep -E "csi-node-register|csi-resizer|csi-provisioner"  | awk '{print$$1}')
	docker rm  $(shell docker ps -a  |grep -E "csi-node-register|csi-resizer|csi-provisioner"  | awk '{print$$1}')
    
	
certs:
	@hack/certs.sh --service carina-controller --namespace default 

run: certs	
	@hack/csi-provisioner.sh  
	@hack/csi-node-register.sh
	
node:  
	 go run ../cmd/carina-node/main.go --http-addr 0.0.0.0:8091 --metrics-addr :8081

controller:
	 go run ../cmd/carina-controller/main.go --http-addr :8090 --metrics-addr :8080