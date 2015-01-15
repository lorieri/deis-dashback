# deis-dashback
deis dashboard backend

## Notes

Moves log collection to deis-dashcollect

# ToDo:

 * pre-process in go before send to redis
 * ~~remove unecessary keys before rename  (ZREMRANGEBYRANK)~~
 * ~~Dockerfile~~
 * ~~create remote_addr rank keys~~
 * ~~deis router PR to add request_time and upstream_response_time~~ (status: waiting approval)
