package main

import (
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func main() {
	var startAgency int
	var endAgency int
	var startCiteNum int
	var endCiteNum int
	var concurrency int
	flag.IntVar(&startAgency, "start-agency", 0, "agency ID")
	flag.IntVar(&endAgency, "end-agency", 200, "agency ID")
	flag.IntVar(&startCiteNum, "start-cite-num", 100000, "range start citation number")
	flag.IntVar(&endCiteNum, "end-cite-num", 999999, "range end citation number")
	flag.IntVar(&concurrency, "concurrency", 16, "number of parallel requests")
	flag.Parse()

	queries := make(chan *Query, 100)
	results := make(chan *Response, 100)
	for i := 0; i < concurrency; i++ {
		go requestWorker(queries, results)
	}
	go func() {
		defer close(queries)
		indices := rand.Perm(endCiteNum - startCiteNum + 1)
		for agencyOffset := 0; agencyOffset < endAgency-startAgency+1; agencyOffset++ {
			for j, i := range indices {
				agency := startAgency + (j+agencyOffset)%(endAgency-startAgency+1)
				citeNum := strconv.Itoa(startCiteNum + i)
				queries <- &Query{Agency: agency, CiteNum: citeNum}
			}
		}
	}()

	var numResults int
	var numFound int

	for result := range results {
		if result.Found {
			numFound++
			fmt.Println(result)
		}
		numResults++
		if numResults%100 == 0 {
			log.Printf("found %d/%d queries", numFound, numResults)
		}
	}

}

type Query struct {
	Agency  int
	CiteNum string
}

type Response struct {
	Query
	Found  bool
	Agency string
	Plate  string
	State  string
	Date   string
	Total  string
	Notes  string
}

func requestWorker(queries <-chan *Query, responses chan<- *Response) {
	for query := range queries {
		q := url.Values{}
		q.Add("agency", strconv.Itoa(query.Agency))
		q.Add("plate", "")
		q.Add("cite", query.CiteNum)
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
		if strings.Contains(string(content), "Sorry, no citations matched your search.") ||
			strings.Contains(string(content), "Agency not found.") {
			responses <- &Response{Query: *query, Found: false}
			continue
		}
		response := &Response{Query: *query, Found: true}
		entries := map[string]*string{
			"DataGrid1_ctl02_LabelAgency":  &response.Agency,
			"DataGrid1_ctl02_LabelPlate":   &response.Plate,
			"DataGrid1_ctl02_LabelState":   &response.State,
			"DataGrid1_ctl02_LabelMessage": &response.Notes,
		}
		contentStr := string(content)
		for labelID, field := range entries {
			*field = parseTableEntry(contentStr, labelID)
		}
		response.Date, response.Total = parseDateAndPrice(contentStr)
		responses <- response
	}
}

func parseTableEntry(content, id string) string {
	starter := `<span id="` + id + `">`
	idx := strings.Index(content, starter)
	if idx == -1 {
		return ""
	}
	content = content[idx+len(starter):]
	endIdx := strings.Index(content, "</span>")
	if endIdx == -1 {
		return ""
	}
	return html.UnescapeString(content[:endIdx])
}

func parseDateAndPrice(content string) (string, string) {
	idx := strings.Index(content, `<span id="DataGrid1_ctl02_LabelState">`)
	if idx == -1 {
		return "", ""
	}
	content = content[idx:]
	content = strings.Replace(content, `<font face="Verdana">`, "", -1)

	var values [2]string
	for i := 0; i < 2; i++ {
		nextTd := strings.Index(content, "<td")
		if nextTd == -1 {
			break
		}
		content = content[nextTd:]
		endTd := strings.Index(content, ">")
		content = content[endTd+1:]
		endEnd := strings.Index(content, "<")
		values[i] = html.UnescapeString(content[:endEnd])
	}
	return values[0], values[1]
}
