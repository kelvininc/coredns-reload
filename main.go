package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Interval        int    `yaml:"interval"`
	ResolvConf      string `yaml:"resolvConf"`
	CoreDNSConfDir  string `yaml:"corednsConfDir"`
	CoreDNSCorefile string `yaml:"corednsCorefile"`
}

// default config values
const (
	defaultInterval        = 5
	defaultResolvConf      = "/systemd-resolve/resolv.conf"
	defaultCoreDNSConfDir  = "/coredns/conf/"
	defaultCoreDNSCorefile = "/etc/coredns/Corefile"
)

// reads the config.yaml and applies defaults if necessary
func loadConfig(configPath string) (*Config, error) {
	config := &Config{
		Interval:        defaultInterval,
		ResolvConf:      defaultResolvConf,
		CoreDNSConfDir:  defaultCoreDNSConfDir,
		CoreDNSCorefile: defaultCoreDNSCorefile,
	}

	if configPath == "" {
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

func fileModified(filePath string, lastModTime time.Time) (bool, time.Time) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("error checking file: %v", err)
		return false, lastModTime
	}
	if fileInfo.ModTime().After(lastModTime) {
		return true, fileInfo.ModTime()
	}
	return false, lastModTime
}

func checkNameserver(resolvConf string) bool {
	data, err := os.ReadFile(resolvConf)
	if err != nil {
		log.Fatalf("failed to read resolv.conf: %v", err)
	}

	nameserverRegex := regexp.MustCompile(`(?m)^\s*nameserver\s+`)
	return nameserverRegex.Match(data)
}

func getSearchDomains(resolvPath string) []string {
	data, err := os.ReadFile(resolvPath)
	if err != nil {
		log.Printf("failed to read resolv.conf: %v", err)
		return nil
	}

	var domains []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "search ") {
			domains = strings.Fields(line)[1:]
			break
		}
	}
	return domains
}

func copyCorefile(config *Config, removeForward bool) {
	internalResolvConf := "/etc/resolv.conf"

	data, err := os.ReadFile(config.CoreDNSCorefile)
	if err != nil {
		log.Fatalf("failed to read coredns config: %v", err)
	}

	if removeForward {
		errors := regexp.MustCompile(`errors`)
		forward := regexp.MustCompile(`(?m:^\s*forward \. .+$)`)

		data = errors.ReplaceAll(data, []byte("# errors"))

		searchDomains := getSearchDomains(internalResolvConf)

		var rewriteRules []string
		for _, domain := range searchDomains {
			rewriteRules = append(rewriteRules, "    rewrite name suffix ."+domain+". .")
		}

		rewriteBlock := strings.Join(rewriteRules, "\n")

		data = forward.ReplaceAll(data, []byte(rewriteBlock))
	}

	fileName := filepath.Base(config.CoreDNSCorefile)
	targetPath := filepath.Join(config.CoreDNSConfDir, fileName)

	err = os.WriteFile(targetPath, data, 0644)
	if err != nil {
		log.Fatalf("failed to write coredns config: %v", err)
	}

	log.Printf("coredns config updated at %s", targetPath)
}

func getProcessPID(processName string) (string, error) {
	cmd := exec.Command("pgrep", "-x", processName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// pgrep can return multiple pids, get the first one
	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(pids) > 0 {
		return pids[0], nil
	}

	return "", fmt.Errorf("CoreDNS process not found")
}

func main() {
	configPath := flag.String("conf", "", "path to the configuration file")
	initFlag := flag.Bool("init", false, "initialize coredns configuration")
	checkFlag := flag.Bool("check", false, "check and update coredns configuration")
	flag.Parse()

	if !*initFlag && !*checkFlag || *initFlag && *checkFlag {
		log.Fatalf("error: you must pass one of -init or -check arguments")
	}

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	if *initFlag {
		if checkNameserver(config.ResolvConf) {
			copyCorefile(config, false)
		} else {
			copyCorefile(config, true)
		}
		log.Println("coredns conf initialized successfully")
		return
	}

	if *checkFlag {
		log.Printf("monitoring resolvconf file '%s' every %d seconds", config.ResolvConf, config.Interval)

		fileInfo, err := os.Stat(config.ResolvConf)
		if err != nil {
			log.Fatalf("error getting file info: %v", err)
		}
		lastModTime := fileInfo.ModTime()

		for {
			modified, newModTime := fileModified(config.ResolvConf, lastModTime)
			if modified {
				if checkNameserver(config.ResolvConf) {
					copyCorefile(config, false)
				} else {
					copyCorefile(config, true)
				}

				corednsPID, err := getProcessPID("/coredns")

				if err != nil {
					log.Fatalf("error getting the pid of coredns process: %v", err)
				}

				out, err := exec.Command("kill", "-SIGUSR1", corednsPID).CombinedOutput()

				if err != nil {
					log.Fatalf("error sending SIGUSR1 signal to pid %s: %s", corednsPID, out)
				}

				log.Printf("signal SIGUSR1 sent to pid %s", corednsPID)
				log.Printf("reloading coredns")

				lastModTime = newModTime
			}
			time.Sleep(time.Duration(config.Interval) * time.Second)
		}
	}
}
