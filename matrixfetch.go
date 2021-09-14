package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	matrixSessionCookieName        = "JSESSIONID"
	matrixRendermapTokenCookieName = "oam.Flash.RENDERMAP.TOKEN"
	urlMatrixLogin                 = "/matrix-v3.7.3.75487/login.jspx"
	urlMatrixMainMenu              = "/matrix-v3.7.3.75487/mainMenu.jsf"
	urlMatrixLogout                = "TODO"

	matrixDebugPrint = false
)

// FetchMatrixEntries returns today's entries available in "Aktuelle Buchungen" in Matrix and the current flexitime balance.
func FetchMatrixEntries(config MatrixConfig) ([]Entry, time.Duration, error) {
	client, err := NewMatrixClient(config)
	if err != nil {
		return nil, 0, err
	}
	defer client.Close()

	entries, err := client.GetEntries()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to retrieve entries: %s", err.Error())
	}

	flexitime, err := client.GetFlexiTime()
	if err != nil {
		return nil, 0, fmt.Errorf("could not retrieve flexitime: %s", err.Error())
	}

	return entries, flexitime, nil
}

// MatrixConfig contains config parameters for Matrix connection and login.
type MatrixConfig struct {
	Host string `json:"host"`
	User string `json:"user"`
	Pass string `json:"pass" jcrypt:"aes"`
}

// DormaClient represents an authorized connection to Matrix.
type MatrixClient struct {
	config          MatrixConfig
	httpClient      *http.Client
	sessionID       string
	rendermapToken  string
	monthDataID     string
	bookingID       string
	lastVisitedPage string
	nextUniqueToken string
	nextViewState   string
}

// NewDormaClient returns a logged in DormaClient.
func NewMatrixClient(config MatrixConfig) (*MatrixClient, error) {
	client := &MatrixClient{
		config: config,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		},
	}

	if err := client.login(); err != nil {
		return nil, fmt.Errorf("login failed: %s", err.Error())
	}
	if err := client.visitSelfService(); err != nil {
		return nil, fmt.Errorf("visit self-service failed: %s", err.Error())
	}
	return client, nil
}

// Close logs out from Matrix and closes the connection.
func (c *MatrixClient) Close() error {
	return c.logout()
}

func (c *MatrixClient) login() error {
	encodedUser := url.QueryEscape(c.config.User)
	encodedPass := url.QueryEscape(c.config.Pass)
	timeZoneName := "Europe/Berlin" //TODO dynamic
	timeZoneOffset := "+01:00"      //TODO dynamic
	encodedTimeZoneName := url.QueryEscape(timeZoneName)
	encodedTimeZoneOffset := url.QueryEscape(timeZoneOffset)
	requestBody := fmt.Sprintf("userid=%s&password=%s&systemLevel=false&timezonename=%s&timezoneoffset=%s&timezonedst=true&loginButton=Anmeldung", encodedUser, encodedPass, encodedTimeZoneName, encodedTimeZoneOffset)

	if _, err := c.postRedirect(urlMatrixLogin, requestBody); err != nil {
		return err
	}
	return nil
}

func (c *MatrixClient) visitSelfService() error {
	requestBody := "uniqueToken=" + c.nextUniqueToken + "&autoScroll=&agmenuform_SUBMIT=1&javax.faces.ViewState=" + c.nextViewState + "&activateMenuItem=mss_root&menuIndex=4&agmenuform%3AassemblyGroupMenu=agmenuform%3AassemblyGroupMenu&data-matrix-treepath=mss_root&agmenuform%3AassemblyGroupMenu_menuid=4"

	if _, err := c.postRedirect(urlMatrixMainMenu, requestBody); err != nil {
		return nil
	}
	return nil
}

func (c *MatrixClient) logout() error {
	return nil
}

// GetEntries returns all entries for the current day.
func (c *MatrixClient) GetEntries() ([]Entry, error) {
	requestBody := "uniqueToken=" + c.nextUniqueToken + "&menuform_SUBMIT=1&autoScroll=&javax.faces.ViewState=" + c.nextViewState + "&activateMenuItem=tim_searchWebBookingMss&menuform%3AmainMenu_mss_root_menuid=" + c.bookingID + "&data-matrix-treepath=mss_root.tim_searchWebBookingMss&menuform%3AmainMenu_mss_root=menuform%3AmainMenu_mss_root"

	body, err := c.postRedirect(c.lastVisitedPage, requestBody)
	if err != nil {
		return nil, err
	}

	pattern := regexp.MustCompile(`title="(Uhrzeit \(SZ\)|Time \(ST\))" class="dateTimeMinuteValue">\s*(\d+):(\d+)\s*</span></td><td role="gridcell" class="tableColumnCenter"><span id="mainbody:editWebBooking:logTable:\d+:logTypeOfBookingTable">([^<]+)</span>`)

	today := time.Now()

	matches := pattern.FindAllStringSubmatch(body, -1)
	entries := make([]Entry, 0)
	for _, m := range matches {
		if len(m) != 5 {
			continue
		}

		hour, _ := strconv.Atoi(m[2])
		minute, _ := strconv.Atoi(m[3])
		date := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, time.Local)

		typeStr := m[4]
		var entryType EntryType
		if strings.Contains(strings.ToLower(typeStr), "kommen") || strings.Contains(strings.ToLower(typeStr), "arrive") || strings.Contains(strings.ToLower(typeStr), "business authorisation") {
			entryType = EntryTypeCome
		} else if strings.Contains(strings.ToLower(typeStr), "gehen") || strings.Contains(strings.ToLower(typeStr), "leave") || strings.Contains(strings.ToLower(typeStr), "hourly absence - end") || strings.Contains(strings.ToLower(typeStr), "system - baend") {
			entryType = EntryTypeLeave
		} else if strings.Contains(strings.ToLower(typeStr), "???bookingtype.1034.name???") {
			// "???BookingType.1034.name???" wird geschrieben, wenn man am Terminal den Kontostand abfragt
			continue
		} else {
			return nil, fmt.Errorf("cannot parse entry type from %q", typeStr)
		}

		entries = append(entries, Entry{Time: date, Type: entryType})
	}

	return entries, nil
}

// GetFlexiTime returns the current flexi time balance.
func (c *MatrixClient) GetFlexiTime() (time.Duration, error) {
	requestBody := "uniqueToken=" + c.nextUniqueToken + "&menuform_SUBMIT=1&autoScroll=&javax.faces.ViewState=" + c.nextViewState + "&activateMenuItem=tim_persMonthlyReconciliation&menuform%3AmainMenu_mss_root_menuid=" + c.monthDataID + "&data-matrix-treepath=mss_root.tim_persMonthlyReconciliation&menuform%3AmainMenu_mss_root=menuform%3AmainMenu_mss_root"

	body, err := c.postRedirect(c.lastVisitedPage, requestBody)
	if err != nil {
		return 0, err
	}

	return c.parseFlexiTime(body)
}

func (c *MatrixClient) parseFlexiTime(body string) (time.Duration, error) {
	pattern := regexp.MustCompile(`<td class="tableColumnRight" title="(Saldo Vortag|Balance previous day)" width="100"><span id="mainbody:editPersRecord:monthrecon:listDynTable:\d+:contentj_id__v_4">\s*(-?)\s*[&nbsp;]*\s*(\d+):(\d+)\s*</span>`)
	matches := pattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("unable to parse current flexi-time balance")
	}

	m := matches[len(matches)-1]

	sign := 1
	if m[2] == "-" {
		sign = -1
	}
	hours, _ := strconv.Atoi(m[3])
	minutes, _ := strconv.Atoi(m[4])
	return time.Duration(sign) * (time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute), nil
}

func (c *MatrixClient) postRedirect(url, body string) (string, error) {
	firstURL := c.config.Host + url
	request, err := http.NewRequest(http.MethodPost, firstURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	c.setCookies(request)
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 302 {
		return "", fmt.Errorf("server returned code %d when 302 was expected", response.StatusCode)
	}

	c.evalCookies(response)

	if len(c.sessionID) == 0 {
		return "", fmt.Errorf("missing Cookie " + matrixSessionCookieName)
	}

	request, err = http.NewRequest(http.MethodGet, c.config.Host+response.Header.Get("Location"), nil)
	if err != nil {
		return "", err
	}
	c.setCookies(request)

	c.lastVisitedPage = response.Header.Get("Location")
	if matrixDebugPrint {
		fmt.Println("lastVisitedPage:", c.lastVisitedPage)
	}

	response, err = c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("server returned code %d when 200 was expected", response.StatusCode)
	}

	buffer, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	body = string(buffer)

	c.evalCookies(response)

	pattern := regexp.MustCompile(`<input type="hidden" name="uniqueToken" value="([^"]*)" />`)
	m := pattern.FindStringSubmatch(body)
	if len(m) != 2 {
		return "", fmt.Errorf("unable to parse unique token")
	}
	c.nextUniqueToken = m[1]
	if matrixDebugPrint {
		fmt.Println("UniqueToken:", c.nextUniqueToken)
	}

	pattern = regexp.MustCompile(`javax.faces.ViewState:\d+" value="([^"]*)"`)
	m = pattern.FindStringSubmatch(body)
	if len(m) != 2 {
		return "", fmt.Errorf("unable to parse view state")
	}
	c.nextViewState = m[1]
	if matrixDebugPrint {
		fmt.Println("ViewState:", c.nextViewState)
	}

	pattern = regexp.MustCompile(`'tim_searchWebBookingMss','menuform:mainMenu_mss_root_menuid':'(\d+)'`)
	m = pattern.FindStringSubmatch(body)
	if len(m) == 2 {
		c.bookingID = m[1]
		if matrixDebugPrint {
			fmt.Println("BookingID:", c.bookingID)
		}
	}

	pattern = regexp.MustCompile(`'tim_persMonthlyReconciliation','menuform:mainMenu_mss_root_menuid':'(\d+)'`)
	m = pattern.FindStringSubmatch(body)
	if len(m) == 2 {
		c.monthDataID = m[1]
		if matrixDebugPrint {
			fmt.Println("MonthDataID:", c.monthDataID)
		}
	}

	return body, nil
}

func (c *MatrixClient) setCookies(request *http.Request) {
	if len(c.sessionID) > 0 {
		request.AddCookie(&http.Cookie{Name: matrixSessionCookieName, Value: c.sessionID})
	}
	if len(c.rendermapToken) > 0 {
		request.AddCookie(&http.Cookie{Name: matrixRendermapTokenCookieName, Value: c.rendermapToken})
	}
	request.AddCookie(&http.Cookie{Name: "timezonedst", Value: "true"})
	request.AddCookie(&http.Cookie{Name: "timezonename", Value: "Europe/Berlin"})
	request.AddCookie(&http.Cookie{Name: "timezoneoffset", Value: "+01:00"})
}

func (c *MatrixClient) evalCookies(response *http.Response) {
	for _, cookie := range response.Cookies() {
		if cookie.Name == matrixSessionCookieName {
			if matrixDebugPrint {
				fmt.Println("SessionID:", cookie.Value)
			}
			c.sessionID = cookie.Value
		}
		if cookie.Name == matrixRendermapTokenCookieName {
			if matrixDebugPrint {
				fmt.Println("RendermapToken:", cookie.Value)
			}
			c.rendermapToken = cookie.Value
		}
	}
}
