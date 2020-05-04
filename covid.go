package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"math/rand"

	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/websocket"
	"github.com/slack-go/slack"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type covidclient struct {
	s     *websocket.Conn
	DocID string
}


func GetData() (OutData, error) {
	od := OutData{
		Confirmed: make(map[string]int),
		Dead:      make(map[string]int),
	}
	for {
		c := NewCovidClient()
		defer c.s.Close()
		c.SendString(fmt.Sprintf(`{"delta":true,"handle":-1,"method":"OpenDoc","params":["%s","","","",false],"id":1,"jsonrpc":"2.0"}`, c.DocID))
		c.SendString(`{"delta":true,"method":"IsPersonalMode","handle":-1,"params":[],"id":2,"jsonrpc":"2.0"}`)
		c.SendString(`{"delta":true,"handle":1,"method":"GetAppLayout","params":[],"id":3,"jsonrpc":"2.0"}`)
		c.SendString(`{"delta":true,"handle":1,"method":"GetObject","params":["RHjRJ"],"id":4,"jsonrpc":"2.0"}`)
		rand.Seed(time.Now().UnixNano())
		bytz := make([]byte, 16)
		rand.Read(bytz)
		lol := base64.StdEncoding.EncodeToString(bytz)
		sendme := fmt.Sprintf(`{"delta":true,"handle":1,"method":"CreateSessionObject","params":[{"qInfo":{"qType":"SelectionObject","qId":"%s"},"qSelectionObjectDef":{}}],"id":5,"jsonrpc":"2.0"}`, strings.ReplaceAll(lol, "=", ""))
		//fmt.Println(sendme)
		c.SendString(sendme)
		c.SendString(`{"delta":true,"handle":1,"method":"CreateSessionObject","params":[{"qInfo":{"qType":"BookmarkList","qId":"MUjSEzzm"},"qBookmarkListDef":{"qType":"bookmark","qData":{"title":"/qMetaDef/title","description":"/qMetaDef/description","sheetId":"/sheetId","selectionFields":"/selectionFields","creationDate":"/creationDate"}}}],"id":6,"jsonrpc":"2.0"}`)
		vals := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true}
		for {
			_, mmsg, _ := c.s.ReadMessage()
			d := json.NewDecoder(bytes.NewReader(mmsg))
			bbuh := GetDocListResp{}
			d.Decode(&bbuh)
			//fmt.Println(buh)
			if bbuh.Error.Message != "" {
				return OutData{}, fmt.Errorf("%d %s", bbuh.ID, bbuh.Error.Message)
			}
			delete(vals, bbuh.ID)
			if len(vals) == 0 {
				break
			}
		}
		c.SendString(`{"delta":true,"handle":2,"method":"GetLayout","params":[],"id":7,"jsonrpc":"2.0"}`)
		_, msg, _ := c.s.ReadMessage()
		d := json.NewDecoder(bytes.NewReader(msg))
		buh := ActualData{}
		d.Decode(&buh)
		if buh.Error.Message != "" {
			panic("no")
		}
		lolstart := 8
		for {
			c.SendString(fmt.Sprintf(`{"delta":true,"handle":2,"method":"GetHyperCubeData","params":["/qHyperCubeDef",[{"qTop":0,"qLeft":0,"qHeight":9,"qWidth":1},{"qTop":0,"qLeft":1,"qHeight":9,"qWidth":1},{"qTop":0,"qLeft":2,"qHeight":9,"qWidth":1}]],"id":%d,"jsonrpc":"2.0"}`, lolstart))
			lolstart++
			_, msg2, _ := c.s.ReadMessage()
			d = json.NewDecoder(bytes.NewReader(msg2))
			buh = ActualData{}
			d.Decode(&buh)
			//fmt.Println(string(msg))
			if buh.Error.Message != "" {
				c.s.Close()
				break
			}
			for i, j := range buh.Result.QDataPages[0].Value[0].QMatrix {
				confirm := buh.Result.QDataPages[0].Value[1].QMatrix[i][0].QNum
				dead := buh.Result.QDataPages[0].Value[2].QMatrix[i][0].QNum
				od.Confirmed[j[0].QText] = confirm
				od.Dead[j[0].QText] = dead
			}
			return od, nil
		}
	}
	return od, fmt.Errorf("Whoops")
}

func NewCovidClient() *covidclient {
	u := url.URL{
		Scheme: "wss",
		Host:   "covid19-data.health.gov.au",
		Path:   "/app/engineData",
	}
	docid := ""
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	c.WriteMessage(websocket.TextMessage, []byte(`{"delta":true,"method":"GetDocList","handle":-1,"params":[],"id":1,"jsonrpc":"2.0"}`))
	for {
		_, msg, _ := c.ReadMessage()
		d := json.NewDecoder(bytes.NewReader(msg))
		buh := GetDocListResp{}
		d.Decode(&buh)
		if buh.ID == 1 {
			// fmt.Println(buh.Result.QDocList[0].Value[0].QDocID)
			docid = buh.Result.QDocList[0].Value[0].QDocID
			u.Path = "/app/" + docid
			c.Close()
			break
		}
	}
	c, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
	return &covidclient{
		s:     c,
		DocID: docid,
	}
}
func (cc *covidclient) SendString(s string) {
	cc.s.WriteMessage(websocket.TextMessage, []byte(s))
}

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}

type GetDocListResp struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Delta   bool   `json:"delta"`
	Error   Error
	Result  struct {
		QDocList []struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value []struct {
				QDocName        string `json:"qDocName"`
				QConnectedUsers int    `json:"qConnectedUsers"`
				QFileTime       int    `json:"qFileTime"`
				QFileSize       int    `json:"qFileSize"`
				QDocID          string `json:"qDocId"`
				QMeta           struct {
					CreatedDate  time.Time   `json:"createdDate"`
					ModifiedDate time.Time   `json:"modifiedDate"`
					Published    bool        `json:"published"`
					PublishTime  time.Time   `json:"publishTime"`
					Privileges   []string    `json:"privileges"`
					Description  string      `json:"description"`
					DynamicColor string      `json:"dynamicColor"`
					Create       interface{} `json:"create"`
					Stream       struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"stream"`
					CanCreateDataConnections bool `json:"canCreateDataConnections"`
				} `json:"qMeta"`
				QLastReloadTime time.Time `json:"qLastReloadTime"`
				QTitle          string    `json:"qTitle"`
				QThumbnail      struct {
					QURL string `json:"qUrl"`
				} `json:"qThumbnail"`
			} `json:"value"`
		} `json:"qDocList"`
	} `json:"result"`
}
type Error struct {
	Code      int    `json:"code"`
	Parameter string `json:"parameter"`
	Message   string `json:"message"`
}
type ActualData struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Delta   bool   `json:"delta"`
	Error   Error
	Result  struct {
		QDataPages []struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value []struct {
				QMatrix [][]struct {
					QText       string `json:"qText"`
					QNum        int    `json:"qNum"`
					QElemNumber int    `json:"qElemNumber"`
					QState      string `json:"qState"`
					QAttrExps   struct {
						QValues []struct {
							QText string `json:"qText"`
							QNum  string `json:"qNum"`
						} `json:"qValues"`
					} `json:"qAttrExps"`
				} `json:"qMatrix"`
				QTails []struct {
					QUp   int `json:"qUp"`
					QDown int `json:"qDown"`
				} `json:"qTails"`
				QArea struct {
					QLeft   int `json:"qLeft"`
					QTop    int `json:"qTop"`
					QWidth  int `json:"qWidth"`
					QHeight int `json:"qHeight"`
				} `json:"qArea"`
			} `json:"value"`
		} `json:"qDataPages"`
	} `json:"result"`
}
type OutData struct {
	Confirmed map[string]int
	Dead      map[string]int
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
	for _, newrow := range new {
		newboi := false
		for _, oldrow := range old {
			if oldrow[1] != newrow[1] {
				newboi = true
				continue
			}
			newboi = false
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
		if newboi {
			d := append(newrow, newrow[2], "NEW DATA")
			deltas = append(deltas, d)
			modified = append(modified, newrow)
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

	// //Australia data
	// response, err := http.Get("https://www.health.gov.au/news/health-alerts/novel-coronavirus-2019-ncov-health-alert/coronavirus-covid-19-current-situation-and-case-numbers")
	// if err == nil {
	// 	defer response.Body.Close()

	/*------------------------------------------------------*/
	// CSTO ADDITIONS
	strayaData, err := GetData()
	if err != nil {
		log.Println(err)
	}

	// tableData = append(tableData, []string{"straya", "location", "cases", "deados"})
	for k, v := range strayaData.Confirmed {
		tableData = append(tableData, []string{"straya", Sanitise(k), fmt.Sprintf("%d", v), currTime})
	}
	// log.Println(tableData)
	/*------------------------------------------------------*/
	lastUpdated = fmt.Sprintf("As of %v there are {%s} confirmed cases across straya and {%s} deads.", currTime, Format(strayaData.Confirmed["Australia"]), Format(strayaData.Dead["Australia"]))
	// Create a goquery document from the HTTP response
	// 	document, err := goquery.NewDocumentFromReader(response.Body)
	// 	if err != nil {
	// 		log.Fatal("Error loading HTTP response body. ", err)
	// 	}

	// 	// tableData = append(tableData, []string{"~Straya", "~Dataz", "~Delta (TBD)"})

	// 	// Get the table data from the stupid fucking table - also fuck HTML
	// 	document.Find(".wrapper .health-table__responsive tr").Each(func(i int, s *goquery.Selection) {
	// 		res := []string{"straya"}
	// 		s.Find("td").Each(func(i int, s *goquery.Selection) {
	// 			d := strings.TrimSpace(s.Text())
	// 			if strings.HasPrefix(d, "*") {
	// 				return
	// 			}
	// 			res = append(res, Sanitise(d))
	// 		})
	// 		if len(res) < 3 {
	// 			return
	// 		}
	// 		res = append(res, currTime)
	// 		tableData = append(tableData, res)
	// 	})
	// 	document.Find(".au-body .main-content .au-callout, .au-body .main-content .field-name-field-link-external, .au-body .main-content .paragraphs-item-content-callout").Each(func(i int, s *goquery.Selection) {
	// 		lastUpdated = fmt.Sprintf("%v%v", lastUpdated, strings.TrimSpace(s.Text()))
	// 	})
	// }
	// log.Println(err)

	//Murica data
	response, err := http.Get("https://www.cdc.gov/coronavirus/2019-ncov/cases-updates/cases-in-us.html")
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
	// fmt.Println(tableData)
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
