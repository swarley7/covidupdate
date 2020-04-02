# covidupdate
a slackbot that updates current covid-19 numbers

# Requires:
- golang
- a computer to run on
- a Slack bot token (should look similar to "xoxb-XXXXXX-XXXXXXXX-XXXXXXX")
- a slack channel and name
- the :remain_indoors: tag to be added to the slack as a custom emoji (use the supplied image file)

# Usage:

`cd /path/to/covidupdate`

`go get -u`

`GOOS=linux GOARCH=amd64 go build covid.go`

`crontab -e # on the server you want it to run from`

Add an entry to run every 10 mins:

`*/10 * * * * /path/to/covid -key "xoxb-XXXXXX-XXXXXXXX-XXXXXXX" -channel covid-19 -file covidtable.csv`
(hint: replace the channel and key with your own details. file is less important; that's where covid will store and check data from)

