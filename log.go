package main

import (
	"github.com/ActiveState/tail"
	"fmt"
	"time"
	"strings"
	"io"
	"github.com/satyrius/gonx"
	"gopkg.in/redis.v2"
	"strconv"
	"os"
)

func main() {

	// start the ticker
	ticker()

	// makes sure log is being read
	for {
		readlog()
		time.Sleep(time.Second * 1)
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
						tx.Rename(v,strings.Replace(v, "last5", "last10", 1))
						unionkeys[strings.Replace(v, "last5", "", 1)] = true
					}

					for _,v := range keys0 {
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

func readlog() {
	// parser
	// wikipedia: HTTP referer (originally a misspelling of referrer)
	format  := `$deis_time $deis_unit: [$level] - [$time_local] - $remote_addr - $remote_user - $status - "$request" - $bytes_sent - "$http_referer" - "$http_user_agent" - "$server_name" - $upstream_addr`

	// redis

	redisServer := getopt("REDIS_SERVER", "127.0.0.1:6379")
        rc := redis.NewClient(&redis.Options{Network: "tcp", Addr: redisServer})


	logFile := getopt("LOG_FILE", "/var/lib/deis/store/logs/deis-router.log")
	// tail 
	location := tail.SeekInfo{Offset: 0, Whence: 2}
	t, err := tail.TailFile(logFile, tail.Config{Follow: true, ReOpen: true, Location: &location })
	for line := range t.Lines {
		// fmt.Println(line.Text)

		parseline := strings.NewReader(line.Text)
		reader := gonx.NewReader(parseline, format)

		// parse line
		for {
			rec, err := reader.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				panic(err)
			}
			upstream_addr, _ := rec.Field("upstream_addr")
			//remote_addr, _ := rec.Field("remote_addr")
			status, _ := rec.Field("status")
			bytes_sent_str, _ := rec.Field("bytes_sent")
			bytes_sent, errleo := strconv.ParseInt(bytes_sent_str, 0, 64)
			if errleo != nil {
				fmt.Printf("%+v\n",errleo)
			}
			// fmt.Printf("%+v\n",bytes_sent)
			http_referer, _ := rec.Field("http_referer")
			request, _ := rec.Field("request")
			// fmt.Printf("%q\n", strings.Split("a,b,c", ",")[0])
			server_name, _ := rec.Field("server_name")
			server_name = strings.Split(server_name, "^")[1]
			server_name = strings.Split(server_name, `\`)[0]
			// ZIncrBy(key string, increment int, member string)

			// global
			rc.ZIncrBy("current_z_top_upstream", 1, upstream_addr)
			rc.ZIncrBy("current_z_top_apps", 1, server_name)
			if !strings.HasPrefix(status, "2") && !strings.HasPrefix(status, "3"){
				rc.ZIncrBy("current_z_top_error_app_status",1,server_name+"_"+status)
				rc.IncrBy("current_k_total_errors", 1)
			}
			rc.IncrBy("current_k_total_bytes", bytes_sent)
			rc.IncrBy("current_k_total_requests", 1)


			// apps
			rc.ZIncrBy("current_z_top_app_upstream_"+server_name,1,upstream_addr)
			rc.ZIncrBy("current_z_top_app_upstream_status_"+server_name,1, status+" - "+upstream_addr)
			rc.ZIncrBy("current_z_top_app_request_"+server_name, 1, request)
			rc.ZIncrBy("current_z_top_app_status_"+server_name, 1, status+" "+request)
			if !strings.HasPrefix(status, "2") && !strings.HasPrefix(status, "3"){
				rc.ZIncrBy("current_z_top_app_error_referer_"+server_name, 1 , status+" "+http_referer)
				rc.ZIncrBy("current_z_top_app_error_request_"+server_name, 1 , status+" "+request)
				rc.IncrBy("current_k_total_app_errors_"+server_name, 1)
			}
			rc.ZIncrBy("current_z_top_app_referer_"+server_name, 1, http_referer)
			//rc.IncrBy("current_k_total_app_bytes"+server_name, bytes_sent)
			rc.IncrBy("current_k_total_app_requests_"+server_name, 1)

		}
	}
	if err != nil {
                panic(err)
        }
}
