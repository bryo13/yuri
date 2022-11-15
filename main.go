package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"github.com/mitchellh/go-homedir"
)

type Response struct {
	Result string
	Err    error
}

// checkNet pings 8.8.8.8 to check if there is an internet connection
func checkNet(r chan Response) {

	cmd := exec.Command("ping", "8.8.8.8")

	var out bytes.Buffer

	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		r <- Response{"", errors.New("no internet")}
		os.Exit(1)
	}

	r <- Response{"Internet", nil}
}

// countDays counts the number of days from day one to today
func countDays() int {
	date := time.Now()

	format := "2006-01-02 15:04:05"
	then, _ := time.Parse(format, "2022-11-13 07:58:06")

	diff := date.Sub(then)

	fmt.Println(int(diff.Hours() / 24))
	return int(diff.Hours() / 24) // number of days
}

// loadDevotional scraps daily devotional text and packages the message as an email
func loadDevotionalEmail() ([]byte, error) {
	dev, err := http.Get(os.Get("WEB"))
	if err != nil {
		return []byte("couldnt scrap web page"), errors.New("couldnt scrap webpage")
	}

	defer dev.Body.Close()

	if dev.StatusCode != 200 {
		return []byte("webpage returned non 200 code"), errors.New("non 200 status code")
	}

	doc, err := goquery.NewDocumentFromReader(dev.Body)
	if err != nil {
		log.Fatal("load devotional err", err)
	}

	var devotional []string
	daysFromStart := countDays()
	subject := fmt.Sprintf("Subject: Day %d of devotional\r\n", daysFromStart)
	devotional = append(devotional, "To:brian_tum@outlook.com\r\n"+subject)
	// Find the review items
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the title
		devotional = append(devotional, s.Text())
	})

	buf := &bytes.Buffer{}
	gob.NewEncoder(buf).Encode(devotional)
	ds := buf.Bytes()
	return []byte(ds), nil
}

// sendEmail sends devotional emails to me if there is an internet connection, daily
func sendEmail(body []byte) {
	godotenv.Load()
	from := os.Getenv("EMAIL")
	password := os.Getenv("PASS")

	to := []string{"brian_tum@outlook.com"}
	host := "smtp.gmail.com"
	port := "587"
	auth := smtp.PlainAuth("", from, password, host)

	if err := smtp.SendMail(host+":"+port, auth, from, to, body); err != nil {
		log.Println("send email", err)
		os.Exit(1)
	}

}

// checkIfEmailIdSent confirms if an email is sent by checking if a folder is created.
// if folder is present email is sent
func checkIfEmailSent() bool {
	home, _ := homedir.Dir()
	location := fmt.Sprintf("%s/%s", home, ".devotional")

	if _, err := os.Stat(location); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func main() {
	// if there is no internet, run the code again after 1 minute
	ticker := time.NewTicker(1 * time.Minute)
	home, _ := homedir.Dir()
	location := fmt.Sprintf("%s/%s", home, ".devotional")

	// check if email is sent - a temp folder would have been created called .devotional
	isSent := checkIfEmailSent()

	body, _ := loadDevotionalEmail()
	response := make(chan Response)

	go checkNet(response)
	net_ok := <-response

	if !isSent {
		for range ticker.C {
			if net_ok.Err == nil {
				sendEmail(body)
				os.Mkdir(location, 0755)
				ticker.Stop()
				break
			}
		}
	} else {
		stat, _ := os.Stat(location)
		timeCreated := stat.ModTime()
		today := time.Now()
		// format := "2006-01-02 15:04:05"

		diff := today.Sub(timeCreated)

		sinceCreated := int(diff.Hours() / 24)

		if sinceCreated < 1 {
			return
		}
		os.Remove(location)
	}

	os.Exit(0)
}
