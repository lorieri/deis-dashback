#!/bin/bash

/bin/redis-server --save "" --maxmemory-policy allkeys-lru &
/back
