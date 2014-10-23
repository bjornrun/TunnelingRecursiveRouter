/*
Tunneling Recursive Router

Copyright (c) 2014 Bjorn Runaker

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package main

import (
	"io"
	"log"
	"fmt"
	"net/http"
	"flag"
	"os"
	"os/exec"
	"bufio"
	"regexp"
	"strings"
	"strconv"
	config "github.com/stvp/go-toml-config"
	sh "github.com/bjornrun/go-sh"
)

var (
	tapname            	 = config.String("tapname", "tap")
	numtap          	 = config.Int("numtap", 1)
	starttap             = config.Int("starttap", 0)
	startport    		 = config.Int("startport", 50025)
	startip     		 = config.String("startip", "10.0.1.136")
	stepip           	 = config.Int("stepip", 4)
	tapdaemon  			 = config.String("tapdaemon", "./tapdaemon")
	listen  			 = config.String("listen", "localhost:18080")

)

const maxTap=256
var cfgFile string
var verbose bool
var cmds        [256]*exec.Cmd
var allocNames  [256]string
var tapNames    [256]string
var ipAddr      [256]string
var port2tap    [256]int
var port2server [256]int
var expression string
var command string
var logfile string
var bQuiet bool
var bVerbose bool
var bDryrun bool


var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr,"\nWeb commands:\nhttp://%s/allocate/<signum>_<instance> - allocate a free port -> assigned IP address\n",*listen)
	fmt.Fprintf(os.Stderr,"http://%s/remove/<signum>_<instance> - remove an allocated port\n",*listen)
	fmt.Fprintf(os.Stderr,"http://%s/port/<signum>_<instance> Show port\n",*listen)
	fmt.Fprintf(os.Stderr,"http://%s/ip/<signum>_<instance> Show IP address\n",*listen)
	fmt.Fprintf(os.Stderr,"http://%s/list - list allocated ports\n",*listen)
	fmt.Fprintf(os.Stderr,"Example of tapmanager.cfg:\ntapname=\"tap\"\nnumtap=1\nstarttap=0\nstartip=\"10.1.1.4\"\nstepip=4\ntapdaemon=\"./tapdaemon\"\nlisten=\"127.0.0.1:18080\"\n")
}

func readLoop(r *bufio.Reader, index int, w http.ResponseWriter) {
	var re = regexp.MustCompile(expression)

	fmt.Println("starting")

	for {
		str, err := r.ReadString('\n')
		fmt.Println(str)

		if len(str) > 0 {
			if !bQuiet { fmt.Print(str) }

			if (re.MatchString(str)) {
				if bVerbose { fmt.Print("*match*") }
			} else
			{
				continue
			}

			cmd := command

			match := re.FindStringSubmatch(str)
			if match == nil {
				if bVerbose { fmt.Println("no match!!") }
				continue
			}

			for i := 0; i < re.NumSubexp(); i++ {
				subre := regexp.MustCompile("<"+re.SubexpNames()[i+1]+">")
				cmd = subre.ReplaceAllString(cmd, match[i+1])

			}

			if bVerbose || bDryrun { fmt.Println("cmd = " + cmd) }

			var serverport, _ = strconv.Atoi(cmd);

			port2server[index] = serverport;

			fmt.Fprintf(w, "{\"Tap\":\"%s\", \"Ip\":\"%s\", \"Port\":%d, \"ServerPort\":%d, \"Status\":\"OK\"}\n", tapNames[index], ipAddr[index], port2tap[index], port2server[index])


			if (!bDryrun) {
				cmdArray := strings.Split(cmd, " ")

				params := make([]interface{}, 0)
				cmd0 := ""
				for index,element := range cmdArray {
					if (index == 0) {
						cmd0 = element
					} else
					{
						params = append(params, element)
					}
				}



				if logfile == "" {
					sh.Command(cmd0, params...).Run()
				} else
				{
					file, err := os.OpenFile(logfile, os.O_RDWR|os.O_APPEND, 0666)
					if err != nil {
						file, err = os.Create(logfile)
						if (err != nil) {
							fmt.Println("Can't write to " + logfile)
							os.Exit(1)
						}
					}
					defer file.Close()
					w := bufio.NewWriter(file)

					c1 := sh.Command(cmd0, params...)
					c1.Stdout = w
					c1.Start()
					c1.Wait()
					wc, ok := c1.Stdout.(io.WriteCloser)
					if ok {
						wc.Close()
					}


					w.Flush()
				}
			}

		}
		if err == io.EOF {
			os.Exit(0)
		}
		if err != nil {
			log.Println("Read Line Error:", err)
			continue
		}


	}
}

func execWatch(i int, cmd *exec.Cmd, w http.ResponseWriter) {
	donec := make(chan error, 1)
	go func() {
		donec <- cmd.Wait()
	}()
	select {
		//      case <-time.After(3 * time.Second):
		//              cmd.Process.Kill()
		//              fmt.Println("timeout")
	case <-donec:
		fmt.Println("done and removed")
		allocNames[i] = ""
		if (cmds[i] != nil) {
			cmds[i] = nil
		}
	}
}

func allocateHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/allocate/"):]
	fmt.Printf("alloc name = %s\n", name)
	for i, line := range allocNames {
		if (line == name) {
			fmt.Fprintf(w, "{\"Tap\":\"%s\", \"Ip\":\"%s\", \"Port\":%d, \"ServerPort\":%d, \"Status\":\"OK\"}\n", tapNames[i], ipAddr[i], port2tap[i], port2server[i])
			return
		}
	}
	for i, line := range allocNames {
		if (line == "") {
			if (i >= *numtap) {
				fmt.Fprintf(w, "{\"Status\":\"FAIL\", \"Reason\":\"Full\"}\n")
				return
			} else
			{
				fmt.Fprintf(w, "{\"Tap\":\"%s\", \"Ip\":\"%s\", \"Port\":%d, \"ServerPort\":%d, \"Status\":\"OK\"}\n", tapNames[i], ipAddr[i], port2tap[i], port2server[i])
				allocNames[i] = name
				cmds[i] = exec.Command(*tapdaemon, tapNames[i], fmt.Sprintf("%d", port2tap[i]))
				cmds[i].Start()
				go execWatch(i, cmds[i], w)
				return
			}
		}
	}
	fmt.Fprintf(w, "{\"Status\":\"FAIL\", \"Reason\":\"Full\"}\n")
}

func removeHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/remove/"):]
	fmt.Printf("remove name = %s\n", name)

	for i, line := range allocNames {
		if (line == name) {
			fmt.Fprintf(w, "{\"Status\":\"OK\"}")
			allocNames[i] = ""
			fmt.Printf("removed\n")
			if (cmds[i] != nil) {
				cmds[i].Process.Kill()
				cmds[i].Wait()
				cmds[i] = nil
			}
			return

		}

	}
	fmt.Fprintf(w, "{\"Status\":\"FAIL\", \"Reason\":\"Not found\"}\n")
}

func portHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/port/"):]
	fmt.Printf("port name = %s\n", name)
	for i, line := range allocNames {
		if (line == name) {
			fmt.Fprintf(w, "{\"Port\":%d, \"Status\":\"OK\"}\n", port2tap[i])
			return
		}
	}
	fmt.Fprintf(w, "{\"Status\":\"FAIL\", \"Reason\":\"Not found\"}\n")
}

func ipHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/ip/"):]
	fmt.Printf("ip name = %s\n", name)
	for i, line := range allocNames {
		if (line == name) {
			fmt.Fprintf(w, "{\"Ip\":\"%s\",\"Status\":\"OK\"}\n", ipAddr[i])
			return
		}
	}
	fmt.Fprintf(w, "{\"Status\":\"FAIL\", \"Reason\":\"Not found\"}\n")

}

func listHandler(w http.ResponseWriter, r *http.Request) {
	for i, line := range allocNames {
		if (line != "") {
			fmt.Fprintf(w, "{\"Name\":\"%s\", \"Tap\":\"%s\", \"Ip\":\"%s\", \"Port\":%d, \"Status\":\"OK\"}\n",line, tapNames[i], ipAddr[i], port2tap[i])
		}

	}
}

func main() {

	expression = "server listening at port (?P<port>\\d+)"
	command = "<port>"
	bVerbose = true
	bQuiet = false


	flag.StringVar(&cfgFile, "c", "tapmanager.cfg", "TAPmanager config setup file")
	flag.BoolVar(&verbose,"v", false, "Verbose")

	flag.Usage = Usage
	flag.Parse()

	if err := config.Parse(cfgFile); err != nil {
		panic(err)
	}

	if  verbose {
		fmt.Printf("TAPmanager\n")
	}

	var ip [4]int
	_, err := fmt.Sscanf(*startip, "%d.%d.%d.%d", &ip[0], &ip[1], &ip[2], &ip[3])
	if err != nil {
		panic(err)
	}
	for i := 0; i < maxTap; i++ {
		tapNames[i] = fmt.Sprintf("%s%1d",*tapname,*starttap+i)
		ipAddr[i] = fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3]+i*(*stepip))
		port2tap[i] = *startport + i
	}

	http.HandleFunc("/allocate/", allocateHandler)
	http.HandleFunc("/remove/", removeHandler)
	http.HandleFunc("/list/", listHandler)
	http.HandleFunc("/ip/", ipHandler)
	http.HandleFunc("/port/", portHandler)
	http.ListenAndServe(*listen, nil)
}


