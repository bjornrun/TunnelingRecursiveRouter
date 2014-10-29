/*
Tunneling Recursice Router Client

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
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package main

import (
	"flag"
	"os"
)

var (
	//	tunnels             = config.String("tunnels", "")
	portStart          = config.Int("portStart", 10000)
	portEnd            = config.Int("portEnd", 65535)
	lockdir            = config.String("lockdir", "/tmp/tunnelsetup/")
	socksStart         = config.Int("proxy.socksStart", 1080)
	socksEnd           = config.Int("proxy.socksEnd", 10800)
	socksActive        = config.Bool("proxy.socksActive", false)
//	proxyServerAddr    = config.String("proxy.address", "10.0.1.136")
//	proxySSHMasterFlag = config.String("proxy.sshmasterflag", "-o \"ControlMaster=yes\" -o \"ControlPath=~/.ssh/%r@%h:%p\"")
//	proxyUser          = config.String("proxy.user", "proxy")
	instance           = config.Int("instance", 0)
//	sshbin             = config.String("ssh", "ssh")
	tunnelbin		   = config.String("ss-tunnel")
	clientbin		   = config.String("ss-client")
	
)

var cfgFile string
var command string
var ctrlSocket string
var tunnelListFile string
var bSocks bool
var bQuiet bool
var socksSocket int
var userName string

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nConfig file:\nportStart = <first port to be used on localhost>\nportEnd = <last port to use on localhost\n[proxy]\nport = <SOCKS proxy to create on localhost. OPTIONAL (used with -s parameter)>\naddress = \"<IP address to proxy. MANDATORY>\"\n")
	fmt.Fprintf(os.Stderr, "user=\"<proxy username. MANDATORY>\"\n")
	fmt.Fprintf(os.Stderr, "ssh=\"<ssh client with full path. Recommended if not using default ssh>\"\n")
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func getLastPortFromString(str string) string {
	re := regexp.MustCompile("([0-9]+)$")
	result := re.FindStringSubmatch(str)
	if len(result) == 2 {
		return result[1]
	}
	return ""
}

func checkTunnelPresent(path string, tunnelType string, port string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	re := regexp.MustCompile(fmt.Sprintf("%s ([0-9]+):([a-z0-9.]+):([0-9]+)", tunnelType))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result := re.FindStringSubmatch(scanner.Text())

		if len(result) == 4 {
			if result[3] == port {
				return true
			}
		}
	}
	return false
}

func getLocalTunnelPort(path string, tunnelType string, port string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	re := regexp.MustCompile(fmt.Sprintf("%s ([0-9]+):([a-z0-9.]+):([0-9]+)", tunnelType))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result := re.FindStringSubmatch(scanner.Text())
		if len(result) == 4 {
			if result[3] == port {
				return result[1]
			}
		}
	}
	return "-1"
}

func saveTunnel2Config(templ string, arg ...string) {
	f, err := os.OpenFile(tunnelListFile, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		f, err = os.OpenFile(tunnelListFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
	}

	defer f.Close()

	switch len(arg) {
	case 1:
		if _, err = f.WriteString(fmt.Sprintf(templ, arg[0])); err != nil {
			panic(err)
		}
	case 2:
		if _, err = f.WriteString(fmt.Sprintf(templ, arg[0], arg[1])); err != nil {
			panic(err)
		}
	case 3:
		if _, err = f.WriteString(fmt.Sprintf(templ, arg[0], arg[1], arg[2])); err != nil {
			panic(err)
		}
	default:
		if _, err = f.WriteString(fmt.Sprintf(templ, arg)); err != nil {
			panic(err)
		}
	}
}

func main() {
	flag.StringVar(&cfgFile, "c", "tunnels.cfg", "Tunnel config setup file")
	flag.StringVar(&command, "e", "help", "Execute command (NOTE: must be last parameter): \n help\n attach\n detach\n config\n forward <local port:ip:remote port>\n remote <remote port:ip:local port>\n autoforward <ip:remote port>\n ")
	flag.BoolVar(&bSocks, "s", false, "Enable SOCKS server on attach")
	flag.BoolVar(&bQuiet, "q", false, "Quiet just print the port number. Used in scripts")
	flag.Usage = Usage
	flag.Parse()

	if !bQuiet {
		fmt.Printf("Tunnel Setup\n")
	}

	if err := config.Parse(cfgFile); err != nil {
		panic(err)
	}
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	userName = usr.Username

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	if (*socksActive) {
		bSocks = true
	}

	ctrlSocket := fmt.Sprintf("%s/.ssh/%s.%s.%d", usr.HomeDir, *proxyServerAddr, hostname, *instance)
	tunnelListFile = fmt.Sprintf("%s/.ssh/%s.%s.%d.txt", usr.HomeDir, *proxyServerAddr, hostname, *instance)


	if command == "help" {
		flag.PrintDefaults()
		os.Exit(0)
	} else if command == "attach" {

		if _, err := os.Stat(ctrlSocket); err == nil {
			cmd := exec.Command(*sshbin, "-O", "check", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), *proxyServerAddr)

			output, err := cmd.CombinedOutput()
			if err != nil {
				if !bQuiet {
					fmt.Println(fmt.Sprint(err) + ": " + string(output))
					os.Remove(ctrlSocket)
					os.Remove(tunnelListFile)
					fmt.Println("Socket connection is removed")
				}
			} else {
				if bQuiet {
					// if running quiet (ie in a script) it is assumed a new master connection is wanted each time
					// os.Exit(0)
					os.Remove(ctrlSocket)
					os.Remove(tunnelListFile)
				} else
				{
					fmt.Printf("Server %s already attached", *proxyServerAddr)
					fmt.Println(string(output))
					os.Exit(1)
				}
			}

		}

		os.Remove(tunnelListFile)

		socksSocket = *socksStart
		if (!bSocks) {
			socksSocket = -1
		}
		var cmd *exec.Cmd


	retryMaster:

		if bSocks {
			cmd = exec.Command(*sshbin, "-o", "ControlMaster=yes", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), "-o", "TCPKeepAlive=yes", "-o", "ServerAliveInterval=60",  "-o", "StrictHostKeyChecking=no", "-o", "ExitOnForwardFailure=yes", "-fNT", "-D", fmt.Sprintf("%d", socksSocket), "-l", *proxyUser, *proxyServerAddr)

		} else {
			cmd = exec.Command(*sshbin, "-o", "ControlMaster=yes", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), "-o", "TCPKeepAlive=yes", "-o", "ServerAliveInterval=60", "-o", "StrictHostKeyChecking=no", "-fNT", "-l", *proxyUser, *proxyServerAddr)

		}


		stdout, err := cmd.StdoutPipe()

		if err != nil {
			log.Fatal(err)
		}

		err = cmd.Start()
		if err != nil {
			log.Fatal(err)
		}

		go io.Copy(os.Stdout, stdout)

		err = cmd.Wait()
		if err != nil {
			if bSocks && socksSocket < *socksEnd {
				socksSocket++
				goto retryMaster
			}
			log.Fatal(err)
		}

		if !bQuiet {
			fmt.Printf("Server %s is now attached\n", *proxyServerAddr)
		}
		if socksSocket > -1 {
			if !bQuiet {
				fmt.Print("Socks server on port ")
			}
			fmt.Printf("%d\n", socksSocket)
			saveTunnel2Config("SOCKS server at %s\n", strconv.Itoa(socksSocket))
		}
		os.Exit(0)
	} else if command == "detach" {
		if _, err := os.Stat(ctrlSocket); os.IsNotExist(err) {
			if bQuiet {
				os.Exit(0)
			}
			fmt.Printf("Server %s already detached", *proxyServerAddr)
			os.Exit(1)
		}

		cmd := exec.Command(*sshbin, "-O", "stop", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), *proxyServerAddr)

		output, err := cmd.CombinedOutput()
		if err != nil {
			if !bQuiet {
				fmt.Println(fmt.Sprint(err) + ": " + string(output))
				os.Remove(ctrlSocket)
				os.Remove(tunnelListFile)
				fmt.Println("Socket connection is removed")
			}
		} else {
			fmt.Println(string(output))
		}

		if !bQuiet {
			fmt.Printf("Server %s is now detached\n", *proxyServerAddr)
		}
		os.Remove(tunnelListFile)
		os.Exit(0)
	} else if command == "forward" {
		if _, err := os.Stat(ctrlSocket); os.IsNotExist(err) {
			if bQuiet {
				fmt.Println("-1")
			} else {
				fmt.Printf("Server %s is not attached\n", *proxyServerAddr)
			}
			os.Exit(1)
		}
		if checkTunnelPresent(tunnelListFile, "Forward", getLastPortFromString(os.Args[len(os.Args)-1])) {
			if bQuiet {
				fmt.Printf("%s\n", getLocalTunnelPort(tunnelListFile, "Forward", getLastPortFromString(os.Args[len(os.Args)-1])))
			} else {
				fmt.Printf("Forward tunnel %s is already active\n", os.Args[len(os.Args)-1])
			}
			os.Exit(0)
		}
		cmd := exec.Command(*sshbin, "-4", "-O", "forward", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), "-L", os.Args[len(os.Args)-1], *proxyServerAddr,
			"-o", "ExitOnForwardFailure=yes")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}
		err = cmd.Start()
		if err != nil {
			log.Fatal(err)
		}

		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)

		err = cmd.Wait()
		if err != nil {

			log.Fatal(err)
		}
		if !bQuiet {
			fmt.Printf("Forward tunnel %s active\n", os.Args[len(os.Args)-1])
		}
		saveTunnel2Config("Forward %s\n", os.Args[len(os.Args)-1])
		os.Exit(0)
	} else if command == "autoforward" {
		if _, err := os.Stat(ctrlSocket); os.IsNotExist(err) {
			if bQuiet {
				fmt.Println("-1")
			} else {
				fmt.Printf("Server %s is not attached\n", *proxyServerAddr)
			}
			os.Exit(1)
		}
		if checkTunnelPresent(tunnelListFile, "Forward", getLastPortFromString(os.Args[len(os.Args)-1])) {
			if bQuiet {
				fmt.Printf("%s\n", getLocalTunnelPort(tunnelListFile, "Forward", getLastPortFromString(os.Args[len(os.Args)-1])))
			} else {
				fmt.Printf("Forward tunnel %s is already active\n", os.Args[len(os.Args)-1])
			}
			os.Exit(0)
		}
		port := *portStart

	retryForward:

		cmd := exec.Command(*sshbin, "-4", "-O", "forward", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), "-L",
			fmt.Sprintf("%d:%s", port, os.Args[len(os.Args)-1]), *proxyServerAddr,
			"-o", "ExitOnForwardFailure=yes")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}
		/*    stderr, err := cmd.StderrPipe()
		     if err != nil {
				fmt.Println("2")
		        log.Fatal(err)
		     }
		*/
		err = cmd.Start()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}

		go io.Copy(os.Stdout, stdout)
		//    go io.Copy(os.Stderr, stderr)

		err = cmd.Wait()
		if err != nil {
			if port < *portEnd {
				port++
				goto retryForward
			}
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}

			log.Fatal(err)
		}
		if bQuiet {
			fmt.Printf("%d\n", port)
		} else {
			fmt.Printf("Forward tunnel %d:%s active\n", port, os.Args[len(os.Args)-1])
		}
		saveTunnel2Config("Forward %s:%s\n",strconv.Itoa(port) ,os.Args[len(os.Args)-1])

		os.Exit(0)
	} else if command == "remote" {
		if _, err := os.Stat(ctrlSocket); os.IsNotExist(err) {
			if bQuiet {
				fmt.Println("-1")
			} else {
				fmt.Printf("Server %s is not attached", *proxyServerAddr)
			}
			os.Exit(1)
		}

		cmd := exec.Command(*sshbin, "-O", "forward", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), "-R", os.Args[len(os.Args)-1], *proxyServerAddr)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}
		err = cmd.Start()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}

		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)

		err = cmd.Wait()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}
		if !bQuiet {
			fmt.Printf("Remote tunnel %s active\n", os.Args[len(os.Args)-1])
		}
		saveTunnel2Config("Remote %s\n", os.Args[len(os.Args)-1])
		os.Exit(0)
	} else if command == "autoremote" {
		if _, err := os.Stat(ctrlSocket); os.IsNotExist(err) {
			if bQuiet {
				fmt.Println("-1")
			} else {
				fmt.Printf("Server %s is not attached", *proxyServerAddr)
			}
			os.Exit(1)
		}
		port := *portStart

	retryRemote:

		cmd := exec.Command(*sshbin, "-4", "-O", "forward", "-o", fmt.Sprintf("ControlPath=%s", ctrlSocket), "-R",
			fmt.Sprintf("%s:%d", os.Args[len(os.Args)-1], port), *proxyServerAddr,
			"-o", "ExitOnForwardFailure=yes")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}
		/*    stderr, err := cmd.StderrPipe()
		     if err != nil {
				fmt.Println("2")
		        log.Fatal(err)
		     }
		*/
		err = cmd.Start()
		if err != nil {
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}
			log.Fatal(err)
		}

		go io.Copy(os.Stdout, stdout)
		//    go io.Copy(os.Stderr, stderr)

		err = cmd.Wait()
		if err != nil {
			log.Fatal(err)
			if port < *portEnd {
				port++
				goto retryRemote
			}
			if bQuiet {
				fmt.Println("-1")
				os.Exit(1)
			}

			log.Fatal(err)
		}
		if bQuiet {
			fmt.Printf("%d\n", port)
		} else {
			fmt.Printf("Remote tunnel %s:%d active\n", os.Args[len(os.Args)-1], port)
		}
		saveTunnel2Config("Remote %s:%d\n", os.Args[len(os.Args)-1], strconv.Itoa(port))
		os.Exit(0)
	} else if command == "config" {
		fmt.Printf("Configuration:\nInstance: %d\nServer: %s\n", *instance, *proxyServerAddr)
		if _, err := os.Stat(ctrlSocket); os.IsNotExist(err) {
			fmt.Printf("Not attached\n")
			os.Exit(0)
		} else {
			fmt.Printf("Attached to Proxy %s\n", *proxyServerAddr)
		}

		fmt.Printf("User: %s\n", userName)

		lines, err := readLines(tunnelListFile)
		if err == nil {
			if len(lines) > 0 {

				fmt.Printf("Tunnels:\n")
				for _, item := range lines {
					fmt.Println(item)
				}
			} else {
				fmt.Println("No active tunnels")
			}
		} else {
			fmt.Println("No active tunnels")
		}
		os.Exit(0)
	} else {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
