#!/bin/bash -xe
docker-compose build
docker-compose run torstaticbuilder /usr/local/go/bin/go run build.go build-all 
docker-compose run torstaticbuilder strip -s /build/tor-static/tor/dist/bin/tor
docker-compose run torstaticbuilder upx -9 /build/tor-static/tor/dist/bin/tor
docker-compose down --rmi all

