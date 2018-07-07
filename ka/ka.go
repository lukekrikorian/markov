package ka

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"

	"golang.org/x/net/publicsuffix"
)

const (
	origin = "https://www.khanacademy.org"
)

var (
	idRegexp    = regexp.MustCompile(`/(\d{10,16})`)
	punctuation = regexp.MustCompile(`[""''\?,!\.]`)
)

// Notes is an array of either comments/replies
type Notes []Note

// Note is the data for a single comment/reply
type Note struct {
	Content string
	Key     string
}

// Feedback holds data for the feedback endpoint
type Feedback struct {
	Feedback []Note
}

// DiscussionRequest is the JSON data for a POST request for discussion
type DiscussionRequest struct {
	Text      string `json:"text"`
	TopicSlug string `json:"topic_slug"`
}

// ProgramPage describes a page of program data
type ProgramPage struct {
	Cursor      string
	Scratchpads []Program
}

// Program contains data on a certain program
type Program struct {
	URL string
	ID  string
}

// Notifications is an array of notifications
type Notifications struct {
	Cursor        string
	Notifications []Notification
}

// Notification is a KA notification
type Notification struct {
	BrandNew        bool
	FeedbackIsReply bool
	Content         string
	URL             string
	ProgramID       string
	ParentKey       string
	Feedback        string
}

// Account is a KA account, which holds a username/password among other things
type Account struct {
	Username string
	Password string
	fkey     string
	jar      *cookiejar.Jar
	client   http.Client
}

// Login logs the account into KA
func (a *Account) Login() error {
	a.client.Get(origin + "/login")  // Make a first request to generate fkey
	location, _ := url.Parse(origin) // Generate a url for grabbing cookies from that domain

	a.fkey = a.jar.Cookies(location)[0].Value // Assign fkey

	formData := url.Values{
		"continue":   {"null"},
		"fkey":       {a.fkey},
		"identifier": {a.Username},
		"password":   {a.Password},
	} // Generate form data

	resp, _ := a.client.PostForm(origin+"/login", formData) // Send login POST request

	if resp.StatusCode == 200 {
		return nil
	}
	return errors.New("error logging in")
}

// SendComment sends a comment (Tips/Thanks) to the given program
func (a *Account) SendComment(programID string, content string) error {
	c := DiscussionRequest{
		Text:      content,
		TopicSlug: "computer-programming",
	}

	b, _ := json.Marshal(c)

	commentURL := fmt.Sprintf("%s/api/internal/discussions/scratchpad/%s/comments", origin, programID)

	req, _ := http.NewRequest("POST", commentURL, bytes.NewReader(b))

	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-ka-fkey", a.fkey)

	resp, _ := a.client.Do(req)

	if resp.StatusCode == 200 {
		return nil
	}
	return errors.New("error sending comment")
}

// SendReply sends a reply to the given comment
func (a *Account) SendReply(parentID string, content string) error {
	r := DiscussionRequest{
		Text:      content,
		TopicSlug: "computer-programming",
	}

	reqBody, _ := json.Marshal(r)

	replyURL := fmt.Sprintf("%s/api/internal/discussions/%s/replies", origin, parentID)

	req, _ := http.NewRequest("POST", replyURL, bytes.NewReader(reqBody))

	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-ka-fkey", a.fkey)

	resp, _ := a.client.Do(req)

	fmt.Println(resp.StatusCode)

	if resp.StatusCode == 200 {
		return nil
	}
	return errors.New("error replying to comment")
}

// GetHotlist returns a ProgramPage with data from the KA hotlist
func (a *Account) GetHotlist() (ProgramPage, error) {
	var programs ProgramPage

	hotlistURL := origin + "/api/internal/scratchpads/top?sort=3&page=0&subject=all&limit=30"
	resp, err := a.client.Get(hotlistURL)
	if err != nil {
		return programs, err
	}

	if resp.StatusCode != 200 {
		return programs, errors.New("error getting hotlist data")
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return programs, err
	}

	err = json.Unmarshal(bodyBytes, &programs)
	if err != nil {
		return programs, nil
	}

	return programs, errors.New("error getting hotlist data")
}

// GetNotes returns an array of notes
func (a *Account) GetNotes(username string, pages int, c chan Notes, discussionType string) {
	for i := 0; i < pages; i++ {
		var reqJSON Notes
		url := fmt.Sprintf("%s/api/internal/user/%s?username=%s&page=%d", origin, discussionType, username, i)
		req, _ := a.client.Get(url)

		reqBytes, _ := ioutil.ReadAll(req.Body)
		json.Unmarshal(reqBytes, &reqJSON)

		c <- reqJSON
	}
	close(c)
}

// GetUnreadNotifs returns a list of unread notifications
func (a *Account) GetUnreadNotifs() Notifications {
	var temp, notifs Notifications
	url := fmt.Sprintf("%s/api/internal/user/notifications/readable?casing=camel", origin)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("x-ka-fkey", a.fkey)

	resp, _ := a.client.Do(req)

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &temp)

	for _, notif := range temp.Notifications {
		if notif.BrandNew {
			matches := idRegexp.FindStringSubmatch(notif.URL)
			notif.ProgramID = matches[1]

			var f Feedback
			tempURL := fmt.Sprintf("%s/api/internal/discussions/scratchpad/%s/comment?qa_expand_key=%s", origin, notif.ProgramID, notif.Feedback)
			tempReq, _ := a.client.Get(tempURL)
			tempBytes, _ := ioutil.ReadAll(tempReq.Body)
			json.Unmarshal(tempBytes, &f)
			notif.ParentKey = f.Feedback[0].Key

			notifs.Notifications = append(notifs.Notifications, notif)
		}
	}

	return notifs
}

// MarkNotifsAsRead sends a request that marks all unread notifications as read
func (a *Account) MarkNotifsAsRead() error {
	url := fmt.Sprintf("%s/api/internal/user/notifications/clear_brand_new", origin)
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("x-ka-fkey", a.fkey)
	resp, _ := a.client.Do(req)

	if resp.StatusCode == 200 {
		return nil
	}
	return errors.New("error marking notifs as read")
}

// NewAccount generates a new KA account
func NewAccount(username string, password string) *Account {
	jar, _ := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	client := http.Client{
		Jar: jar,
	}
	return &Account{
		Username: username,
		Password: password,
		jar:      jar,
		client:   client,
	}
}

// GenerateIDs extracts all program IDs into their structs
func (p *ProgramPage) GenerateIDs() {
	for i := range p.Scratchpads {
		matches := idRegexp.FindStringSubmatch(p.Scratchpads[i].URL)
		p.Scratchpads[i].ID = matches[1]
	}
}

// Strip removes punctuation from a note
func (n *Note) Strip() {
	n.Content = punctuation.ReplaceAllString(n.Content, "")
}
