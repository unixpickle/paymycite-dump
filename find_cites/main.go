package main

import (
	"flag"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func main() {
	var agency int
	var startCiteNum int
	var endCiteNum int
	var concurrency int
	flag.IntVar(&agency, "agency", 105, "agency ID")
	flag.IntVar(&startCiteNum, "start-cite-num", 600000, "range start citation number")
	flag.IntVar(&endCiteNum, "end-cite-num", 700000, "range end citation number")
	flag.IntVar(&concurrency, "concurrency", 16, "number of parallel requests")
	flag.Parse()

	ch := make(chan string, 100)
	for i := 0; i < concurrency; i++ {
		go requestWorker(agency, ch)
	}
	for _, i := range rand.Perm(endCiteNum - startCiteNum + 1) {
		ch <- strconv.Itoa(startCiteNum + i)
	}
}

func requestWorker(agency int, citeNums <-chan string) {
	for citeNum := range citeNums {
		q := url.Values{}
		q.Add("agency", strconv.Itoa(agency))
		q.Add("plate", "")
		q.Add("cite", citeNum)
		q.Add("state", "")
		urlStr := "https://www.paymycite.com/SearchAgency.aspx?" + q.Encode()
		resp, err := http.Get(urlStr)
		if err != nil {
			log.Println("ERROR:", err)
			continue
		}
		content, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Println("ERROR:", err)
			continue
		}
		if !strings.Contains(string(content), "Sorry, no citations matched your search.") {
			log.Println("FOUND:", citeNum)
		}
	}
}
