#!/bin/bash

/bin/redis-server &

while true
do
	/log.go
	sleep 1
done
