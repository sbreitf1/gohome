package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

const (
	matrixSessionCookieName        = "JSESSIONID"
	matrixRendermapTokenCookieName = "oam.Flash.RENDERMAP.TOKEN"
	urlMatrixLogin                 = "/login.jspx"
	urlMatrixMainMenu              = "/mainMenu.jsf"

	matrixDebugPrint  = false
	matrixOutputFiles = false
)

var (
	matrixVersionURL = "/matrix"
)

// FetchMatrixEntries returns today's entries available in "Aktuelle Buchungen" in Matrix and the current flexitime balance.
func FetchMatrixEntries(config MatrixConfig) ([]Entry, time.Duration, error) {
	client, err := NewMatrixClient(config)
	if err != nil {
		return nil, 0, err
	}
	defer client.Close()

	verbosePrint("get entries")
	entries, err := client.GetEntries()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to retrieve entries: %s", err.Error())
	}

	verbosePrint("get flexi time")
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

	verbosePrint("logging in")
	if err := client.login(); err != nil {
		return nil, fmt.Errorf("login failed: %s", err.Error())
	}
	verbosePrint("visit self service page")
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

	if err := c.detectRedirectURI(); err != nil {
		return err
	}

	if _, err := c.postRedirect(matrixVersionURL+urlMatrixLogin, requestBody); err != nil {
		return err
	}
	return nil
}

func (c *MatrixClient) detectRedirectURI() error {
	resp, err := c.httpClient.Get(c.absoluteURL(matrixVersionURL + urlMatrixLogin))
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		parts := strings.Split(resp.Header.Get("Location"), "/")
		if len(parts) != 3 {
			return fmt.Errorf("unexpected redirect url for login: %s", resp.Header.Get("Location"))
		}
		matrixVersionURL = "/" + parts[1]
		verbosePrint("detected matrix url %q", matrixVersionURL)
	}

	return nil
}

func (c *MatrixClient) visitSelfService() error {
	requestBody := "uniqueToken=" + c.nextUniqueToken + "&autoScroll=&agmenuform_SUBMIT=1&javax.faces.ViewState=" + c.nextViewState + "&activateMenuItem=mss_root&menuIndex=4&agmenuform%3AassemblyGroupMenu=agmenuform%3AassemblyGroupMenu&data-matrix-treepath=mss_root&agmenuform%3AassemblyGroupMenu_menuid=_c3d3c76c-a976-4d74-a147-db02d56ddb08|4"

	if _, err := c.postRedirect(matrixVersionURL+urlMatrixMainMenu, requestBody); err != nil {
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
	if matrixOutputFiles {
		if err := os.WriteFile("entries.html", []byte(body), os.ModePerm); err != nil {
			return nil, fmt.Errorf("output entries file: %s", err.Error())
		}
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(body); err != nil {
		return nil, fmt.Errorf("read xml string element: %s", err.Error())
	}

	tableData := doc.FindElement("//*[@id='mainbody:editWebBooking:logTable_data']")
	tableRows := tableData.FindElements("//tr[@data-ri]")

	today := time.Now()

	timeStampRegex := regexp.MustCompile(`\s*(\d+):(\d+)\s*`)

	entries := make([]Entry, 0)
	for _, row := range tableRows {
		timeSpan := row.ChildElements()[0].ChildElements()[0]
		if timeSpan == nil {
			return nil, fmt.Errorf("could not find time span")
		}

		timeStamp := timeSpan.Text()
		m := timeStampRegex.FindStringSubmatch(timeStamp)

		hour, _ := strconv.Atoi(m[1])
		minute, _ := strconv.Atoi(m[2])
		date := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, time.Local)

		if hour == 0 && minute == 0 {
			verbosePrint("ignore booking %q at 00:00", m[4])
			continue
		}

		typeStr := row.ChildElements()[1].ChildElements()[0].Text()
		typeStr = strings.ToLower(typeStr)
		var entryType EntryType
		if strings.Contains(typeStr, "kommen") ||
			strings.Contains(typeStr, "arrive") ||
			strings.Contains(typeStr, "business authorisation") {
			entryType = EntryTypeCome
		} else if strings.Contains(typeStr, "gehen") ||
			strings.Contains(typeStr, "leave") ||
			strings.Contains(typeStr, "hourly absence - end") ||
			strings.Contains(typeStr, "hourly absence end") ||
			strings.Contains(typeStr, "system - baend") {
			if strings.Contains(typeStr, "sequence error") {
				continue
			}
			entryType = EntryTypeLeave
		} else if strings.Contains(typeStr, "???bookingtype.1034.name???") {
			// "???BookingType.1034.name???" wird geschrieben, wenn man am Terminal den Kontostand abfragt
			verbosePrint("found strange booking type: %q", typeStr)
			continue
		} else if strings.Contains(typeStr, "valid until") {
			verbosePrint("found strange booking type: %q", typeStr)
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
	if matrixOutputFiles {
		if err := os.WriteFile("flexitime.html", []byte(body), os.ModePerm); err != nil {
			return 0, fmt.Errorf("output flexitime file: %s", err.Error())
		}
	}

	return c.parseFlexiTime(body)
}

func (c *MatrixClient) parseFlexiTime(body string) (time.Duration, error) {
	doc := etree.NewDocument()
	err := doc.ReadFromString(body)
	if err != nil {
		return 0, fmt.Errorf("unable to flexi-time balance html")
	}

	tableData := doc.FindElements("//*[@title='Balance previous day']")
	if len(tableData) == 0 {
		return 0, fmt.Errorf("unable to parse current flexi-time balance")
	}

	lastElement := tableData[len(tableData)-1]
	if len(lastElement.ChildElements()) == 0 {
		return 0, fmt.Errorf("unable to find flexi-time balance in element childs")
	}

	child := lastElement.ChildElements()[0]
	text := child.Text()

	isNegative := false
	if strings.Contains(text, "-") {
		isNegative = true
		text = strings.ReplaceAll(text, "-", "")
	}

	splitString := strings.Split(text, ":")
	if len(splitString) != 2 {
		return 0, fmt.Errorf("unexpected time format found in flexi-time balance")
	}

	hours, _ := strconv.Atoi(splitString[0])
	minutes, _ := strconv.Atoi(splitString[1])

	if isNegative {
		return -1 * (time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute), nil
	}

	return (time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute), nil
}

func (c *MatrixClient) absoluteURL(url string) string {
	return strings.TrimRight(c.config.Host, "/") + "/" + strings.TrimLeft(url, "/")
}

func (c *MatrixClient) postRedirect(url, body string) (string, error) {
	firstURL := c.absoluteURL(url)
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
	if (response.StatusCode < 300) || (399 < response.StatusCode) {
		return "", fmt.Errorf("server returned code %d when 3xx was expected", response.StatusCode)
	}

	c.evalCookies(response)

	if len(c.sessionID) == 0 {
		return "", fmt.Errorf("missing Cookie " + matrixSessionCookieName)
	}

	if response.Header.Get("Location") == "favoritePage.jsf" {
		// this happens when a custom start page is selected. force redirect to main menu instead
		response.Header.Set("Location", matrixVersionURL+urlMatrixMainMenu)
	}

	request, err = http.NewRequest(http.MethodGet, c.absoluteURL(response.Header.Get("Location")), nil)
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
