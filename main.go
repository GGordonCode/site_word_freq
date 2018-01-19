// This program finds the most frequently occurring words of a
// specified minimum length for a given site.  It is essentailly a
// web crawler that makes its best effort to stay within the hostname
// of the original site.  On a given page, it both scans for text, for
// which it builds a frequncy histogram, plus it extracts the "href"
// links for further processing.  At the end, the accumulated word count
// results for all sites are sorted, with the most frequent ones displayed.
//
// Architecturally it uses the following elements:
// - A configurable fixed number of goroutines.  This is important
// to be able to scale a backend service without rebuilding it.
// - Rich error reporting per goroutine.  This is accomplished by
// sending a struct which contains an error field in addition to the
// input parameters into the task channel.  Using this technique, we
// can clearly sort out which errors are tied to which URLs.
//
// The program uses two channels, one for the goroutines to read URLs
// to process, and another for the results to be sent back to the main
// processing loop.  We use a looping and counting techique that is used
// to determine when we're done processing.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

var (
	concurrency = flag.Int("concurrency", 5,
		"number of active concurrent goroutines")
	minLen   = flag.Int("min_len", 10, "the minimum word length to track")
	totWords = flag.Int("tot_words", 10, "show the top 'this many' words")
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr,
			"usage: %s [-concurrency #] [-min_len #] <start url>\n", os.Args[0])
		os.Exit(1)
	}

	startURL := flag.Arg(0)
	surl, err := url.Parse(startURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The url '%s' is not syntactically valid\n",
			startURL)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	finder := newWordFinder(surl)

	go func() {
		// Shutdown cleanup on termination signal (SIGINT and SIGTERM for now).
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Println(<-ch)
		cancel()
	}()

	finder.run(ctx)
	showStatus(finder)
}

func showStatus(finder *WordFinder) {
	errs := finder.getErrors()
	if errs == nil {
		fmt.Printf("No errors occurred in run.\n")
	} else {
		for _, r := range finder.records {
			fmt.Printf("'%s': error occurred: %v\n", r.url, r.err)
		}
	}

	res := finder.getResults()
	fmt.Printf("top %d word totals:\n", *totWords)
	for i, kv := range res {
		fmt.Printf("[%d] %s: %d\n", i+1, kv.key, kv.value)
	}
}
