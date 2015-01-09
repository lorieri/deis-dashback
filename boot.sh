#!/bin/bash

/bin/redis-server &

while true
do
	go run log.go
	sleep 1
done
