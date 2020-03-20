package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/go-ntlmssp"
)

const (
	sessionCookieName      = "ASP.NET_SessionId"
	urlDormaLogin          = "/scripts/login.aspx"
	urlDormaLogout         = "/scripts/login.aspx?sessiontimedout=2"
	urlDormaEntries        = "/scripts/buchungen/buchungsdata2.aspx?mode=0"
	urlDormaFlexiTime      = "/scripts/data3.aspx?mode=0"
	urlAnwesenheitsTableau = "/scripts/ze-stellen/abtdata.aspx?mode=25"
	urlAbwesenheitsliste   = "/scripts/ze-stellen/abtdata.aspx?mode=376"

	// EntryTypeCome denotes an entry when entering the company.
	EntryTypeCome EntryType = "come"
	// EntryTypeLeave denotes an entry when leaving the company.
	EntryTypeLeave EntryType = "leave"
	// EntryTypeTrip denotes an entry for a short business trip.
	EntryTypeTrip EntryType = "trip"
)

// FetchDormaEntries returns today's entries available in "Aktuelle Buchungen" in Dorma and the current flexitime balance.
func FetchDormaEntries(config DormaConfig) ([]Entry, time.Duration, []Colleague, error) {
	client, err := NewDormaClient(config)
	if err != nil {
		return nil, 0, nil, err
	}
	defer client.Close()

	entries, err := client.GetEntries()
	if err != nil {
		return nil, 0, nil, fmt.Errorf("failed to retrieve entries: %s", err.Error())
	}

	flexitime, err := client.GetFlexiTime()
	if err != nil {
		return nil, 0, nil, fmt.Errorf("could not retrieve flexitime: %s", err.Error())
	}

	colleagues, err := client.GetPresentColleagues()
	if err != nil {
		return nil, 0, nil, fmt.Errorf("could not retrieve colleagues: %s", err.Error())
	}

	return entries, flexitime, colleagues, nil
}

// DormaConfig contains config parameters for Dorma connection and login.
type DormaConfig struct {
	Host string `json:"host"`
	User string `json:"user"`
	Pass string `json:"pass" jcrypt:"aes"`
}

// DormaClient represents an authorized connection to Dorma.
type DormaClient struct {
	config     DormaConfig
	httpClient *http.Client
	sessionID  string
}

// NewDormaClient returns a logged in DormaClient.
func NewDormaClient(config DormaConfig) (*DormaClient, error) {
	client := &DormaClient{
		config: config,
		httpClient: &http.Client{
			Transport: ntlmssp.Negotiator{
				RoundTripper: &http.Transport{},
			},
		},
	}

	if err := client.login(); err != nil {
		return nil, err
	}
	return client, nil
}

// Close logs out from Dorma and closes the connection.
func (c *DormaClient) Close() error {
	return c.logout()
}

func (c *DormaClient) login() error {
	if _, err := c.get(urlDormaLogin); err != nil {
		return err
	}

	if len(c.sessionID) == 0 {
		return fmt.Errorf("missing Cookie " + sessionCookieName)
	}

	return nil
}

func (c *DormaClient) logout() error {
	_, err := c.get(urlDormaLogout)
	return err
}

// GetEntries returns all entries for the current day.
func (c *DormaClient) GetEntries() ([]Entry, error) {
	body, err := c.get(urlDormaEntries)
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile(`<td class="td-tabelle">\s*(&nbsp;)?(\d*)\.?(\d*)\.?(\d*)\s*</td>\s*<td class="td-tabelle">\s*(\d+):(\d+)\s*</td>\s*<td class="td-tabelle">\s*([^<]+?)\s*</td>`)

	// the date is often omitted for repeated values -> save last date to set it to empty entries
	var lastYear, lastMonth, lastDay int

	matches := pattern.FindAllStringSubmatch(body, -1)
	entries := make([]Entry, 0)
	for _, m := range matches {
		if len(m) != 8 {
			continue
		}

		if len(m[2]) > 0 && len(m[3]) > 0 && len(m[4]) > 0 {
			day, _ := strconv.Atoi(m[2])
			month, _ := strconv.Atoi(m[3])
			year, _ := strconv.Atoi(m[4])
			lastYear = year
			lastMonth = month
			lastDay = day
		}

		if lastYear == 0 {
			return nil, fmt.Errorf("missing date for first entry")
		}

		hour, _ := strconv.Atoi(m[5])
		minute, _ := strconv.Atoi(m[6])
		date := time.Date(lastYear, time.Month(lastMonth), lastDay, hour, minute, 0, 0, time.Local)

		typeStr := m[7]
		var entryType EntryType
		if strings.Contains(strings.ToLower(typeStr), "kommen") {
			entryType = EntryTypeCome
		} else if strings.Contains(strings.ToLower(typeStr), "gehen") {
			entryType = EntryTypeLeave
		} else if strings.Contains(strings.ToLower(typeStr), "dienstgang") {
			if hour == 0 && minute == 0 {
				// just ignore full-day entry "DE Dienstgang"
				continue

			} else {
				entryType = EntryTypeTrip
			}
		} else if strings.Contains(strings.ToLower(typeStr), "heimarbeit") {
			// just ignore entry "HE Heimarbeit"
			continue
		} else if strings.Contains(strings.ToLower(typeStr), "krankheit") {
			// just ignore entry "KO/KM Krankheit ohne/mit Krankenschein"
			continue
		} else {
			return nil, fmt.Errorf("cannot parse entry type from %q", typeStr)
		}

		entries = append(entries, Entry{Time: date, Type: entryType})
	}

	return entries, nil
}

// GetFlexiTime returns the current flexi time balance.
func (c *DormaClient) GetFlexiTime() (time.Duration, error) {
	body, err := c.get(urlDormaFlexiTime)
	if err != nil {
		return 0, err
	}

	pattern := regexp.MustCompile(`<input\s+type="hidden"\s+name="glz"\s+id="glz"\s+value="\s*(-?)\s*[&nbsp;]*\s*(\d+):(\d+)"\s*>`)
	m := pattern.FindStringSubmatch(body)
	if len(m) != 4 {
		return 0, fmt.Errorf("unable to parse current flexi-time balance")
	}

	sign := 1
	if m[1] == "-" {
		sign = -1
	}
	hours, _ := strconv.Atoi(m[2])
	minutes, _ := strconv.Atoi(m[3])
	return time.Duration(sign) * (time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute), nil
}

// Colleague represents a colleague with name and status.
type Colleague struct {
	LoggedIn     bool
	Name         string
	InHomeOffice bool
}

// GetPresentColleagues returns the state of all currently visible colleagues.
func (c *DormaClient) GetPresentColleagues() ([]Colleague, error) {
	c1, err := c.getColleaguesFromAnwesenheitsTableau()
	if err != nil {
		return nil, err
	}

	c2, err := c.getColleaguesFromAbwesenheitsliste()
	if err != nil {
		return nil, err
	}

	// iterate over colleagues from Anwesenheitstableau (c1),
	// can't range though because we might need to modify the
	// InHomeOffice field from inside the loop
	for i := 0; i < len(c1); i++ {
		a := &c1[i]

		// look through colleagues on Abwesenheitsliste (c2),
		// if a colleagues is logged in on Anwesenheitstableau but also listed
		// on the Abwesenheitsliste, they most likely are in home office
		for _, b := range c2 {
			if a.LoggedIn && b == a.Name {
				a.InHomeOffice = true
			}
		}
	}

	return c1, nil
}

func (c *DormaClient) getColleaguesFromAnwesenheitsTableau() ([]Colleague, error) {
	body, err := c.get(urlAnwesenheitsTableau)
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile(`<td class="td-tabelle-fett" style="border: 1px solid #(.+); border-left: 0px;border-right: 0px;">&nbsp;&nbsp;(.+),(.+)</td>`)
	matches := pattern.FindAllStringSubmatch(body, -1)

	var colleagues []Colleague

	for _, m := range matches {
		loggedIn := (m[1] == "00CC00")
		name := strings.TrimSpace(m[3]) + " " + strings.TrimSpace(m[2])
		colleagues = append(colleagues, Colleague{loggedIn, name, false})
	}

	return colleagues, nil
}

func (c *DormaClient) getColleaguesFromAbwesenheitsliste() ([]string, error) {
	body, err := c.get(urlAbwesenheitsliste)
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile(`<td class="td-tabelle">(.+), (.+)</td>`)
	matches := pattern.FindAllStringSubmatch(body, -1)

	var names []string

	for _, m := range matches {
		name := strings.TrimSpace(m[2]) + " " + strings.TrimSpace(m[1])
		names = append(names, name)
	}

	return names, nil
}

func (c *DormaClient) get(url string) (string, error) {
	request, err := http.NewRequest("GET", c.config.Host+url, nil)
	if err != nil {
		return "", err
	}
	if len(c.sessionID) > 0 {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: c.sessionID})
	}
	request.SetBasicAuth(c.config.User, c.config.Pass)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("server returned code %d", response.StatusCode)
	}

	for _, cookie := range response.Cookies() {
		if cookie.Name == sessionCookieName {
			c.sessionID = cookie.Value
		}
	}

	buffer := bytes.NewBuffer([]byte{})
	if _, err := io.Copy(buffer, response.Body); err != nil {
		return "", err
	}

	return buffer.String(), nil
}
