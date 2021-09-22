package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/fatih/color"
)


func banner() {
	fmt.Println(color.YellowString("=================================================="))
	fmt.Println(color.HiBlueString(`
	██████╗  ██████╗ ███████╗████████╗███╗   ███╗███████╗ ██████╗ 
	██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝████╗ ████║██╔════╝██╔════╝ 
	██████╔╝██║   ██║███████╗   ██║   ██╔████╔██║███████╗██║  ███╗
	██╔═══╝ ██║   ██║╚════██║   ██║   ██║╚██╔╝██║╚════██║██║   ██║
	██║     ╚██████╔╝███████║   ██║   ██║ ╚═╝ ██║███████║╚██████╔╝
	╚═╝      ╚═════╝ ╚══════╝   ╚═╝   ╚═╝     ╚═╝╚══════╝ ╚═════╝` + " By @KevSecurity_ "))
	fmt.Println(color.BlueString("Scans URLs for Post Message Event Listeners."))
	fmt.Println("Credits: @divadbate for inspiring me with https://github.com/raverrr/plution")
	fmt.Println(color.YellowString("==================================================\n"))
}

var output string
var concurrency int
var silent bool

func main() {
	log.SetFlags(0) // supress date and time on each line
	flag.BoolVar(&silent, "silent", false, "--> Output (Will only output vulnerable URLs)"+"\n")
	flag.StringVar(&output, "o", devNull(), "--> Output (Will only output vulnerable URLs)"+"\n")
	flag.IntVar(&concurrency, "c", 5, "--> Number of concurrent threads (default 5)"+"\n")
	flag.Parse()

	if !silent{
		banner()
	}

	// create the output file
	file, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}
	consoleWriter := bufio.NewWriter(file)

	scanner := bufio.NewScanner(os.Stdin)
	jobs := make(chan string)

	// Chromeless options
	copts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("ignore-certificate-errors", true),
	)
	ectx, ecancel := chromedp.NewExecAllocator(context.Background(), copts...)
	defer ecancel()
	pctx, pcancel := chromedp.NewContext(ectx)
	defer pcancel()
	if err := chromedp.Run(pctx); err != nil {
		// start the browser to ensure we end up making new tabs in an
		// existing browser instead of making a new browser each time.
		// see: https://godoc.org/github.com/chromedp/chromedp#NewContext
		fmt.Fprintf(os.Stderr, "error starting browser: %s\n", err)
		return
	}

	// Concurrency
	var messageEvents []string
	var url string
	var wgroup sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wgroup.Add(1)
		go func() {
			for requestURL := range jobs {

				url = requestURL+hasQuery(requestURL)
				ctx, cancel := context.WithTimeout(pctx, time.Second*30)
				ctx, _ = chromedp.NewContext(ctx)

				err := chromedp.Run(ctx,
					chromedp.Navigate(url),
					chromedp.EvaluateAsDevTools(jsCode(), &messageEvents),
				)

				if len(messageEvents) > 0 {
					for index, listener := range unique(messageEvents) {
						log.Printf("%s: %v", color.GreenString("[+] ") + requestURL, color.GreenString("Potential!"))
						log.Printf("%s: %v", index, listener)
						consoleWriter.WriteString(requestURL + "\n")
						consoleWriter.Flush()
					}
				}

				if err != nil && !silent {
					fmt.Println(color.RedString("[-]"), requestURL, color.RedString(err.Error()))
				}

				cancel()
			}
			wgroup.Done()
		}()
	}

	// Reading input
	for scanner.Scan() {
		jobs <- scanner.Text()
	}
	close(jobs)
	wgroup.Wait()
}

// Does the URL contain a query already?
func hasQuery(url string) string {
	var Qmark = regexp.MustCompile(`\?`)
	var p = ""
	if Qmark.MatchString(url) {
		p = "&"

	} else {
		p = "?"
	}
	return p
}

func jsCode() string {
	return `
		(function _showEvents(events) {
		  let rs = []
		  for (let evt of Object.keys(events)) {
			  console.log(evt + " ----------------> " + events[evt].length);
			  for (let i=0; i < events[evt].length; i++) {
                if (events[evt][i].type == 'message')
				    rs.push(events[evt][i].listener.toString())
			  }
		  }
		  return rs
		})(getEventListeners(window))`
}

func unique (s []string) []string {
	unique := make(map[string]bool, len(s))
	us := make([]string, len(unique))
	for _, elem := range s {
		if len(elem) != 0 {
			if !unique[elem] {
				us = append(us, elem)
				unique[elem] = true
			}
		}
	}

	return us
}

func devNull() string{
	if runtime.GOOS == "windows" {
		return "NUL"
	}
	return "/dev/null"
}