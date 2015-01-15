package main

import (
	"fmt"
	"time"
	"strings"
	"gopkg.in/redis.v2"
	"os"
)

func main() {

	// start the ticker
	ticker()

	// to use ticker as go func()
	for {
		time.Sleep(time.Second * 1000)
	}
}

// copied from github.com/deis
func getopt(name, dfault string) string {
	value := os.Getenv(name)
	if value == "" {
		value = dfault
	}
	return value
}

func ticker() {

	ticker := time.NewTicker(time.Second * 5)
	go func() {

		redisServer := getopt("REDIS_SERVER", "127.0.0.1:6379")

		for t := range ticker.C {
			fmt.Println("Tick at", t)

			client := redis.NewClient(&redis.Options{Network: "tcp", Addr: redisServer})
			incr := func(tx *redis.Multi) ([]redis.Cmder, error) {

				return tx.Exec(func() error {

					keys5, _ := client.Keys("last5*").Result()
					keys10, _ := client.Keys("last10*").Result()
					keys0, _ := client.Keys("current*").Result()
					keysunion, _ := client.Keys("union*").Result()

					unionkeys := make(map[string]bool)

					for _,v := range keys10 {
						tx.Del(v)
					}

					for _,v := range keysunion {
						tx.Del(v)
					}

					for _,v := range keys5 {
						if strings.HasPrefix(v, "_z"){
							tx.ZRemRangeByRank(v, 0, -30)
						}
						tx.Rename(v,strings.Replace(v, "last5", "last10", 1))
						unionkeys[strings.Replace(v, "last5", "", 1)] = true
					}

					for _,v := range keys0 {
                                                if strings.HasPrefix(v, "_z"){
                                                        tx.ZRemRangeByRank(v, 0, -30)
                                                }
						tx.Rename(v,strings.Replace(v, "current", "last5", 1))
						unionkeys[strings.Replace(v, "current", "", 1)] = true
					}

					unioncmd := `redis.call('zunionstore',KEYS[1],2,ARGV[1],ARGV[2]) return 0`
					incrbycmd := `redis.call('incrby', KEYS[1],redis.call('get',ARGV[1])) return 0`

					for k,_ := range unionkeys {
						if strings.HasPrefix(k, "_z") {
							tx.Eval(unioncmd, []string{"union"+k}, []string{"last5"+k,"last10"+k})
						}

						if strings.HasPrefix(k, "_k") {
							tx.Eval(incrbycmd, []string{"union"+k}, []string{"last5"+k})
							tx.Eval(incrbycmd, []string{"union"+k}, []string{"last10"+k})
						}

						if strings.HasPrefix(k, "_s") {
							tx.Rename("last5"+k, "union"+k)
						}
					}

					return nil
				})
			}

			tx := client.Multi()
			defer tx.Close()

			for {
			    cmds, err := incr(tx)
			    if err == redis.TxFailedErr {
				continue
			    } else if err != nil {
				panic(err)
			    }
			    fmt.Println(cmds, err)
			    break
			}


		}
	}()
}
