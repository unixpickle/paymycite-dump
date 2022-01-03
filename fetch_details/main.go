package main

import (
	"encoding/csv"
	"flag"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/unixpickle/essentials"
)

func main() {
	var inputFile string
	var outputFile string
	var concurrency int
	flag.StringVar(&inputFile, "input", "output.csv", "input file, scraped by find_cites")
	flag.StringVar(&outputFile, "output", "details.csv", "output file with extra details")
	flag.IntVar(&concurrency, "concurrency", 16, "number of parallel requests")
	flag.Parse()

	inputs := readRecords(inputFile)
	outputs := make(chan *CitationInfo)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			detailWorker(inputs, outputs)
		}()
	}

	go func() {
		wg.Wait()
		close(outputs)
	}()

	writeRecords(outputFile, outputs)
}

func readRecords(inputFile string) <-chan *CitationInfo {
	in, err := os.Open(inputFile)
	essentials.Must(err)
	defer in.Close()
	r := csv.NewReader(in)
	records, err := r.ReadAll()
	essentials.Must(err)
	if len(records) == 0 {
		essentials.Die("no input records")
	}
	if len(records[0]) != 8 {
		essentials.Die("invalid size of input records:", len(records[0]))
	}
	res := make(chan *CitationInfo, len(records))
	for _, csvRow := range records {
		ci := &CitationInfo{}
		for i, fieldValue := range csvRow {
			*ci.FieldPtrs()[i] = fieldValue
		}
		res <- ci
	}
	close(res)
	return res
}

func writeRecords(path string, records <-chan *CitationInfo) {
	f, err := os.Create(path)
	essentials.Must(err)
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write(CSVFieldNames)
	w.Flush()
	essentials.Must(w.Error())
	count := 0
	for rec := range records {
		w.Write(rec.Fields())
		w.Flush()
		essentials.Must(w.Error())
		count++
		log.Printf("done %d records", count)
	}
}

func detailWorker(inputs <-chan *CitationInfo, outputs chan<- *CitationInfo) {
	for ci := range inputs {
		essentials.Must(fillDetails(ci))
		outputs <- ci
	}
}

type Violation struct {
	Code        string
	Amount      string
	Description string
}

type CitationInfo struct {
	// Fields from the input file.
	Agency     string
	CiteNum    string
	AgencyName string
	Plate      string
	State      string
	Date       string
	Total      string
	Notes      string

	// Filled in details.
	Location  string
	Time      string
	Officer   string
	VIN       string
	VIN4      string
	Make      string
	Model     string
	Color     string
	TabMOYR   string
	Permit    string
	Violation Violation
}

func (c *CitationInfo) Fields() []string {
	ptrs := c.FieldPtrs()
	res := make([]string, len(ptrs))
	for i, p := range ptrs {
		res[i] = *p
	}
	return res
}

func (c *CitationInfo) FieldPtrs() []*string {
	return []*string{
		&c.Agency,
		&c.CiteNum,
		&c.AgencyName,
		&c.Plate,
		&c.State,
		&c.Date,
		&c.Total,
		&c.Notes,
		&c.Location,
		&c.Time,
		&c.Officer,
		&c.VIN,
		&c.VIN4,
		&c.Make,
		&c.Model,
		&c.Color,
		&c.TabMOYR,
		&c.Permit,
		&c.Violation.Code,
		&c.Violation.Amount,
		&c.Violation.Description,
	}
}

func fillDetails(c *CitationInfo) error {
	q := url.Values{}
	q.Add("agency", c.AgencyName)
	q.Add("cite", c.CiteNum)
	q.Add("platestate", "")
	q.Add("citedate", c.Date)
	q.Add("citebal", "")
	q.Add("S1", "")
	q.Add("S2", "")
	q.Add("S3", "")
	q.Add("S4", "")
	q.Add("SearchType", "1")
	u := "https://www.paymycite.com/OnlineContest.aspx?" + q.Encode()
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	contentStr := string(content)

	fields := map[string]*string{
		"txtVioLocation": &c.Location,
		"txtCiteTime":    &c.Time,
		"txtOfficer":     &c.Officer,
		"txtVIN":         &c.VIN,
		"txtVIN4":        &c.VIN4,
		"lblMake":        &c.Make,
		"lblModel":       &c.Model,
		"lblColor":       &c.Color,
		"txtTabMOYR":     &c.TabMOYR,
		"txtOrgPermit":   &c.TabMOYR,
	}
	for labelID, field := range fields {
		*field = fieldValue(contentStr, labelID)
	}
	c.Violation = violationInfo(contentStr)

	return nil
}

func fieldValue(content, id string) string {
	content = strings.Replace(content, `<b><font face="verdana" size="2">`, "", -1)
	idx := strings.Index(content, `<span id="`+id+`"`)
	if idx == -1 {
		return ""
	}
	content = content[idx:]
	idx = strings.Index(content, ">")
	if idx == -1 {
		return ""
	}
	content = content[idx+1:]
	idx = strings.Index(content, "<")
	if idx == -1 {
		return ""
	}
	return html.UnescapeString(content[:idx])
}

func violationInfo(content string) Violation {
	content = strings.Replace(content, `<font face="Verdana" size="2">`, "", -1)
	idx := strings.Index(content, "dgViolation")
	if idx == -1 {
		return Violation{}
	}
	content = content[idx:]
	idx = strings.Index(content, "</tr><tr>")
	if idx == -1 {
		return Violation{}
	}
	content = content[idx:]

	var fields [3]string
	for i := 0; i < 3; i++ {
		idx := strings.Index(content, "<td")
		if idx == -1 {
			break
		}
		content = content[idx:]
		idx = strings.Index(content, ">")
		if idx == -1 {
			break
		}
		content = content[idx+1:]
		idx = strings.Index(content, "<")
		if idx == -1 {
			break
		}
		fields[i] = html.UnescapeString(content[:idx])
	}
	return Violation{
		Code:        fields[0],
		Amount:      fields[1],
		Description: fields[2],
	}
}

var CSVFieldNames = []string{
	"Agency",
	"CiteNum",
	"AgencyName",
	"Plate",
	"State",
	"Date",
	"Total",
	"Notes",
	"Location",
	"Time",
	"Officer",
	"VIN",
	"VIN4",
	"Make",
	"Model",
	"Color",
	"TabMOYR",
	"Permit",
	"Violation.Code",
	"Violation.Amount",
	"Violation.Description",
}
