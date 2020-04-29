package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/slack-go/slack"
)

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}

func Format(n int) string {
	in := strconv.FormatInt(int64(n), 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}

func WriteCSVData(td [][]string, filename string) {
	file, err := os.Create(filename)
	checkError("Cannot create file", err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, value := range td {
		err := writer.Write(value)
		checkError("Cannot write to file", err)
	}
}

func GetCSVData(filename string) ([][]string, error) {
	csvfile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	// Parse the file
	r := csv.NewReader(csvfile)
	//r := csv.NewReader(bufio.NewReader(csvfile))
	res := [][]string{}
	// Iterate through the records
	for {
		// Read each record from csv
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		res = append(res, record)
	}
	return res, nil
}

func Equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func compareDataSets(old, new [][]string) ([][]string, [][]string) {
	modified := [][]string{}
	deltas := [][]string{}
	for _, oldrow := range old {
		for _, newrow := range new {
			if oldrow[1] != newrow[1] {
				continue
			}
			o, err := strconv.Atoi(oldrow[2])
			if err != nil {
				log.Fatal(err)
			}
			n, err := strconv.Atoi(newrow[2])
			if err != nil {
				log.Fatal(err)
			}
			t0, err := time.Parse("2006-01-02_15:04:05", oldrow[len(oldrow)-1])
			if err != nil {
				log.Fatal(err)
			}
			t1, err := time.Parse("2006-01-02_15:04:05", newrow[len(newrow)-1])
			if err != nil {
				log.Fatal(err)
			}
			timeD := t1.Sub(t0)
			// log.Printf("Debug: %v: old: [%v] new: [%v] - Diff [%v]\n", newrow[0], o, n, n-o)
			d := []string{}
			if n-o == 0 {
				d = append(oldrow, fmt.Sprintf("%v", n-o), timeD.String())
				modified = append(modified, oldrow)
			} else {
				d = append(newrow, fmt.Sprintf("%v", n-o), timeD.String())
				modified = append(modified, newrow)

			}
			// This is the category
			deltas = append(deltas, d) // No change;
			break
		}
	}
	return modified, deltas
}

func Sanitise(s string) string {
	d := strings.Replace(s, "Number of ", "", 1)
	d = strings.ReplaceAll(strings.TrimSpace(d), " ", "_")
	d = strings.Replace(d, ",", "", -1)
	return strings.ToLower(d)
}

func Pad(s string, n int) string {
	r := n - len(s)
	if n-len(s) < 1 {
		r = n
	}
	if len(s) > n-3 {
		return s[:n-4] + "... "
	}
	return fmt.Sprintf("%s%s", s, strings.Repeat(" ", r))
}

func tabularise(table [][]string) []slack.Block {

	msg := slack.NewBlockMessage()
	tMap := map[string][][]string{}
	for _, t := range table {
		tMap[t[0]] = append(tMap[t[0]], t)
	}
	var sectionBlock slack.SectionBlock
	for k, v := range tMap {
		textInfo := slack.NewTextBlockObject("plain_text", k, false, false)
		sectionBlock = *slack.NewSectionBlock(textInfo, nil, nil, slack.SectionBlockOptionBlockID("boner-"+k))
		msg = slack.AddBlockMessage(msg, sectionBlock)
		header := fmt.Sprintf("|%v|%v|%v|", Pad("title", 20), Pad("count", 20), "difference_over_time")
		divider := "+--------------------------------------------------------------+"
		out := fmt.Sprintf("```%v\n%v", header, divider)
		for _, row := range v {
			category := row[1]
			delta, err := strconv.Atoi(row[len(row)-2])
			if err != nil {
				log.Println(err)
			}
			deltaTime := row[len(row)-1]
			count, err := strconv.Atoi(row[2])
			if err != nil {
				log.Println(err)
				continue
			}
			out = fmt.Sprintf("%v\n|%v|%v|", out, Pad(category, 20), Pad(Format(count), 20))
			if delta > 0 {
				out = fmt.Sprintf("%v%v|", out, Pad(fmt.Sprintf("+%v in %v", Format(delta), deltaTime), 20))
			} else if delta < 0 {
				out = fmt.Sprintf("%v%v|", out, Pad(fmt.Sprintf("%v in %v", Format(delta), deltaTime), 20))
			} else if delta == 0 {
				out = fmt.Sprintf("%v%v|", out, Pad(" ", 20))
			}

		}
		out += "```"
		d := slack.NewTextBlockObject("mrkdwn", out, false, false)
		dataSectionBlock := *slack.NewSectionBlock(d, nil, nil, slack.SectionBlockOptionBlockID(fmt.Sprintf("boner-%v-data", k)))
		msg = slack.AddBlockMessage(msg, dataSectionBlock)
		msg = slack.AddBlockMessage(msg, slack.NewDividerBlock())
		// currT = oldT
	}

	// b := &slack.TextBlockObject{Type: "plain_text", Text: t[1]}
	// b2 := &slack.TextBlockObject{Type: "plain_text", Text: t[2]}

	return msg.Blocks.BlockSet
}

func main() {
	apiKey := flag.String("key", "", "Supply the bot key")
	channelName := flag.String("channel", "devtrash", "channel to post to")
	covidFile := flag.String("file", "covidtable.csv", "file to store data in")
	// deltaFile := flag.String("dfile", "lastgenerated.txt", "file to store delta timestamp in")

	flag.Parse()
	currTime := time.Now().Format("2006-01-02_15:04:05")

	log.Println("Auth using: ", *apiKey, *channelName)
	api := slack.New(*apiKey)
	// Find the user to post as.
	authTest, err := api.AuthTest()
	if err != nil {
		fmt.Printf("Error getting channels: %s\n", err)
		return
	}
	log.Println("Posting as ", authTest.User)
	tableData := [][]string{}
	lastUpdated := ""

	//Australia data
	response, err := http.Get("https://www.health.gov.au/news/health-alerts/novel-coronavirus-2019-ncov-health-alert/coronavirus-covid-19-current-situation-and-case-numbers")
	if err == nil {
		defer response.Body.Close()

		// Create a goquery document from the HTTP response
		document, err := goquery.NewDocumentFromReader(response.Body)
		if err != nil {
			log.Fatal("Error loading HTTP response body. ", err)
		}

		// tableData = append(tableData, []string{"~Straya", "~Dataz", "~Delta (TBD)"})

		// Get the table data from the stupid fucking table - also fuck HTML
		document.Find(".wrapper .health-table__responsive tr").Each(func(i int, s *goquery.Selection) {
			res := []string{"straya"}
			s.Find("td").Each(func(i int, s *goquery.Selection) {
				d := strings.TrimSpace(s.Text())
				if strings.HasPrefix(d, "*") {
					return
				}
				res = append(res, Sanitise(d))
			})
			if len(res) < 3 {
				return
			}
			res = append(res, currTime)
			tableData = append(tableData, res)
		})
		document.Find(".au-body .main-content .au-callout, .au-body .main-content .field-name-field-link-external, .au-body .main-content .paragraphs-item-content-callout").Each(func(i int, s *goquery.Selection) {
			lastUpdated = fmt.Sprintf("%v%v", lastUpdated, strings.TrimSpace(s.Text()))
		})
	}
	// log.Println(err)

	//Murica data
	response, err = http.Get("https://www.cdc.gov/coronavirus/2019-ncov/cases-updates/cases-in-us.html")
	if err == nil {
		defer response.Body.Close()

		// Create a goquery document from the HTTP response
		document, err := goquery.NewDocumentFromReader(response.Body)
		if err != nil {
			log.Fatal("Error loading HTTP response body. ", err)
		}

		// tableData = append(tableData, []string{"~Murica", "~Dataz", "~Delta (TBD)"})
		labels := []string{"total_cases", "total_deaths"}

		document.Find("div.callouts-container div.callout span.count").Each(func(i int, s *goquery.Selection) {
			num := s.Text()
			num = strings.Replace(num, ",", "", -1)
			num = strings.Replace(num, "\ufeff", "", -1)
			res := []string{"murica", Sanitise(labels[i]), Sanitise(num), currTime}
			tableData = append(tableData, res)
		})
	}
	// log.Println(err)

	//NZ data
	response, err = http.Get("https://www.health.govt.nz/our-work/diseases-and-conditions/covid-19-novel-coronavirus/covid-19-current-cases")
	if err == nil {
		defer response.Body.Close()

		// Create a goquery document from the HTTP response
		document, err := goquery.NewDocumentFromReader(response.Body)
		if err != nil {
			log.Fatal("Error loading HTTP response body. ", err)
		}

		// tableData = append(tableData, []string{"", "", ""})

		// tableData = append(tableData, []string{"~SheepBois", "~Dataz", "~Delta (TBD)"})
		// Get the table data from the stupid fucking table - also fuck HTML
		document.Find("tbody").Each(func(i int, s *goquery.Selection) {
			// We only want the first table
			if i > 0 {
				return
			}
			s.Find("tr").Each(func(i int, s *goquery.Selection) {
				res := []string{"sheep"}
				s.Find("th").Each(func(i int, s *goquery.Selection) {

					// This is now the header / title
					res = append(res, Sanitise(s.Text()))
				})
				s.Find("td").Each(func(i int, s *goquery.Selection) {

					// We can discard the second column too - it's the 'last 24 hours thing'
					if (i+1)%2 == 0 {
						return
					}
					res = append(res, Sanitise(s.Text()))
				})
				res = append(res, currTime)
				tableData = append(tableData, res)
			})
		})
	}
	// log.Println(err)
	exit := true

	oldData, err := GetCSVData(*covidFile)
	if err != nil || len(oldData) == 0 {
		exit = false
		oldData = tableData
	}
	modified, deltas := compareDataSets(oldData, tableData)
	for _, i := range deltas {
		if i[len(i)-2] != "0" {
			exit = false
			break
		}
	}
	if exit {
		log.Fatal("no new data; breakin out")
	}
	WriteCSVData(modified, *covidFile)

	msg := tabularise(deltas)

	//lastUpdated
	attachment := slack.Attachment{
		Pretext: fmt.Sprintf(">%v", lastUpdated),
	}

	targetChan := slack.Channel{}
	channels, err := api.GetChannels(false)
	if err != nil {
		log.Fatal(err)
	}
	for _, channel := range channels {
		if channel.Name != *channelName {
			continue
		}
		targetChan = channel
		break
	}
	slack.OptionDebug(true)

	channelID, timestamp, err := api.PostMessage(targetChan.ID, slack.MsgOptionAsUser(true), slack.MsgOptionText(":remain_indoors: *Daily update of COVID-19 cases in AU* :remain_indoors:", false), slack.MsgOptionCompose(slack.MsgOptionAttachments(attachment), slack.MsgOptionBlocks(msg...)))
	if err != nil {
		log.Fatal(channelID, timestamp, err)
	}
	log.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
}
