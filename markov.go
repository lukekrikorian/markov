package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"markov/chain"
	"markov/ka"
)

const (
	firstAmount = 15
	length      = 2
	oops        = "Cristianop1"
)

var (
	runs = 0
	acc  *ka.Account
)

type markovData struct {
	Chain       *chain.Chain
	CommentsMap map[string]string
	Amount      int
	Name        string
}

var replies = markovData{
	Chain:       chain.NewChain(length),
	CommentsMap: make(map[string]string),
	Amount:      firstAmount,
	Name:        "replies",
}

var comments = markovData{
	Chain:       chain.NewChain(length),
	CommentsMap: make(map[string]string),
	Amount:      firstAmount,
	Name:        "comments",
}

func main() {

	var userData struct {
		Username string
		Password string
	}
	b, _ := ioutil.ReadFile("./userData.json")
	json.Unmarshal(b, &userData)

	acc = ka.NewAccount(userData.Username, userData.Password)
	acc.Login()
	fmt.Println("Logged in!")

	fmt.Println("Gathering startup notes...")
	replies.gatherNotes()
	comments.gatherNotes()

	for {

		if (runs%6) == 0 && runs > 1 {
			fmt.Println("Sending hotlist comment...")
			hotlist, _ := acc.GetHotlist()
			hotlist.GenerateIDs()

			randIndex := rand.Intn(len(hotlist.Scratchpads))
			randLength := rand.Intn(20) + 30

			randomProgram := hotlist.Scratchpads[randIndex]
			randomComment := comments.Chain.Generate(randLength)

			acc.SendComment(randomProgram.ID, randomComment)
			fmt.Println(randomProgram.URL)
		}

		fmt.Println("Getting notifications...")
		notifs := acc.GetUnreadNotifs()
		if len(notifs.Notifications) > 0 {
			fmt.Println("Found new notifications...")
		} else {
			fmt.Println("No new notifications found...")
		}
		for _, notif := range notifs.Notifications {
			if notif.FeedbackIsReply {
				fmt.Println("Replying to notif on", notif.ProgramID)
				acc.SendReply(notif.ParentKey, replies.Chain.Generate((rand.Intn(30) + 10)))
			}
		}
		fmt.Println("Marking all notifications as read...")
		acc.MarkNotifsAsRead()

		replies.getLatestData()
		comments.getLatestData()

		runs++
		fmt.Println("Sleeping...")
		time.Sleep(time.Minute * 10)
	}
}

func (m *markovData) gatherNotes() {
	fileName := fmt.Sprintf("./%s.json", m.Name)
	file, _ := os.OpenFile(fileName, 0x2, os.ModeAppend)
	if file == nil {
		fmt.Println("File", fileName, "was not found, creating it...")
		file, _ = os.Create(fileName)
		channel := make(chan ka.Notes, m.Amount)
		go acc.GetNotes(oops, m.Amount, channel, m.Name)
		for notes := range channel {
			for _, note := range notes {
				note.Strip()
				m.Chain.AddComment(note.Content)
				m.CommentsMap[note.Key] = note.Content
			}
		}

		byteData, _ := json.MarshalIndent(m.CommentsMap, "", "\t")
		file.Write(byteData)
	} else {
		fmt.Println("File", fileName, "was found, reading...")
		reader := bufio.NewReader(file)
		fileBytes, _ := ioutil.ReadAll(reader)
		json.Unmarshal(fileBytes, &m.CommentsMap)
		for i := range m.CommentsMap {
			m.Chain.AddComment(m.CommentsMap[i])
		}
	}
	defer file.Close()
}

func (m *markovData) getLatestData() {

	fmt.Println("Gathering latest", m.Name, "data...")
	changed := false

	fileName := fmt.Sprintf("./%s.json", m.Name)
	file, _ := os.OpenFile(fileName, 0x2, os.ModeAppend)
	defer file.Close()

	channel := make(chan ka.Notes, 2)
	go acc.GetNotes(oops, 2, channel, m.Name)
	for notes := range channel {
		for _, note := range notes {
			note.Strip()
			if _, ok := m.CommentsMap[note.Key]; !ok {
				m.CommentsMap[note.Key] = note.Content
				changed = true
			}
		}
	}

	if changed {
		fmt.Println("New data found, writing to file...")
		mapBytes, _ := json.MarshalIndent(m.CommentsMap, "", "\t")
		file.Write(mapBytes)
	} else {
		fmt.Println("No new data found...")
	}
}
